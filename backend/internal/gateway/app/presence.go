package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	presenceLeaseTTL      = 45 * time.Second
	websocketPingInterval = 15 * time.Second
)

var updatePresenceScript = redis.NewScript(`
local sessions = KEYS[1]
local presence = KEYS[2]
local version = KEYS[3]
local lastState = KEYS[4]
local now = tonumber(ARGV[1])
local expiresAt = tonumber(ARGV[2])
local connectionID = ARGV[3]
local action = ARGV[4]
local leaseMillis = tonumber(ARGV[5])

redis.call('ZREMRANGEBYSCORE', sessions, '-inf', now)
local before = redis.call('ZCARD', sessions)

if action == 'online' then
  redis.call('ZADD', sessions, expiresAt, connectionID)
  redis.call('PEXPIRE', sessions, leaseMillis)
  local hadPresence = redis.call('GET', presence) == 'online'
  local previousState = redis.call('GET', lastState)
  redis.call('SET', presence, 'online', 'PX', leaseMillis)
  if before == 0 and not hadPresence then
    redis.call('SET', lastState, 'online')
    return redis.call('INCR', version)
  end
  if previousState ~= 'online' then
    redis.call('SET', lastState, 'online')
    if previousState == 'offline' then
      return redis.call('INCR', version)
    end
  end
  return 0
end

redis.call('ZREM', sessions, connectionID)
redis.call('ZREMRANGEBYSCORE', sessions, '-inf', now)
local after = redis.call('ZCARD', sessions)
if after == 0 then
  redis.call('DEL', presence)
  if redis.call('GET', lastState) == 'online' then
    redis.call('SET', lastState, 'offline')
    return redis.call('INCR', version)
  end
  return 0
end

redis.call('SET', presence, 'online', 'PX', leaseMillis)
redis.call('PEXPIRE', sessions, leaseMillis)
if redis.call('GET', lastState) ~= 'online' then
  redis.call('SET', lastState, 'online')
  return redis.call('INCR', version)
end
return 0
`)

var readPresenceScript = redis.NewScript(`
local status = redis.call('GET', KEYS[1])
local version = redis.call('GET', KEYS[2])
return {status or 'offline', version or '0'}
`)

type presenceSnapshot struct {
	Status  string `json:"status"`
	Version int64  `json:"version"`
}

type presenceEvent struct {
	Type    string         `json:"type"`
	Payload presenceUpdate `json:"payload"`
	Ts      int64          `json:"ts"`
}

type presenceUpdate struct {
	UserID  string `json:"user_id"`
	Status  string `json:"status"`
	Version int64  `json:"version"`
}

func (a *App) updatePresenceConnection(ctx context.Context, userID, connectionID string, online bool) (int64, error) {
	if a.redisClient == nil {
		return 0, errors.New("redis client is unavailable")
	}

	action := "offline"
	if online {
		action = "online"
	}
	now := time.Now()
	keys := []string{
		"presence:sessions:" + userID,
		"presence:user:" + userID,
		"presence:version:sequence",
		"presence:last-state:" + userID,
	}
	version, err := updatePresenceScript.Run(ctx, a.redisClient, keys,
		now.UnixMilli(),
		now.Add(presenceLeaseTTL).UnixMilli(),
		connectionID,
		action,
		presenceLeaseTTL.Milliseconds(),
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("update presence for user %s: %w", userID, err)
	}
	return version, nil
}

func (a *App) getUserPresence(ctx context.Context, userID string) (presenceSnapshot, error) {
	if a.redisClient == nil {
		return presenceSnapshot{Status: "unknown"}, errors.New("redis client is unavailable")
	}

	values, err := readPresenceScript.Run(ctx, a.redisClient, []string{
		"presence:user:" + userID,
		"presence:version:sequence",
	}).Slice()
	if err != nil {
		return presenceSnapshot{Status: "unknown"}, fmt.Errorf("read presence for user %s: %w", userID, err)
	}
	if len(values) != 2 {
		return presenceSnapshot{Status: "unknown"}, errors.New("presence snapshot returned an invalid result")
	}
	status, ok := values[0].(string)
	if !ok {
		return presenceSnapshot{Status: "unknown"}, fmt.Errorf("presence status for user %s has unexpected type %T", userID, values[0])
	}
	version, err := strconv.ParseInt(fmt.Sprint(values[1]), 10, 64)
	if err != nil {
		return presenceSnapshot{Status: "unknown"}, fmt.Errorf("parse presence version for user %s: %w", userID, err)
	}

	return presenceSnapshot{Status: status, Version: version}, nil
}

func (a *App) publishPresenceChange(ctx context.Context, userID, status string, version int64) error {
	if a.redisClient == nil {
		return errors.New("redis client is unavailable")
	}
	if a.services == nil {
		return errors.New("friend service is unavailable")
	}

	friends, err := a.services.ListFriends(ctx, userID)
	if err != nil {
		return fmt.Errorf("list friends for presence update: %w", err)
	}
	payload, err := json.Marshal(presenceEvent{
		Type: "presence",
		Payload: presenceUpdate{
			UserID:  userID,
			Status:  status,
			Version: version,
		},
		Ts: time.Now().UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("encode presence update: %w", err)
	}

	var publishErrors []error
	for _, friend := range friends {
		if err := a.redisClient.Publish(ctx, "friend:user:"+friend.ID, payload).Err(); err != nil {
			publishErrors = append(publishErrors, fmt.Errorf("publish presence to friend %s: %w", friend.ID, err))
		}
	}
	return errors.Join(publishErrors...)
}

func (a *App) publishPresenceVersion(ctx context.Context, userID, status string, version int64) error {
	if version == 0 {
		return nil
	}
	return a.publishPresenceChange(ctx, userID, status, version)
}
