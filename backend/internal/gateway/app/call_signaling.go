package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const callSessionTTL = 45 * time.Second

type callSession struct {
	CallID         string `json:"callId"`
	ConversationID string `json:"conversationId"`
	CallerID       string `json:"callerId"`
	CalleeID       string `json:"calleeId"`
	CallType       string `json:"callType"`
}

func (a *App) handleCallSignal(ctx context.Context, senderID string, raw []byte) error {
	if a.redisClient == nil || a.services == nil {
		return errors.New("call signaling unavailable")
	}
	var event struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &event); err != nil || !strings.HasPrefix(event.Type, "call.") || len(event.Payload) == 0 {
		return errors.New("invalid call signal")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(event.Payload, &fields); err != nil || fields == nil {
		return errors.New("invalid call signal payload")
	}
	callID := readJSONText(fields["callId"])
	if _, err := uuid.Parse(callID); err != nil {
		return errors.New("invalid call id")
	}

	if event.Type == "call.invite" {
		return a.createCallSession(ctx, senderID, callID, fields)
	}
	if event.Type != "call.accept" && event.Type != "call.reject" &&
		event.Type != "call.offer" && event.Type != "call.answer" &&
		event.Type != "call.ice" && event.Type != "call.end" && event.Type != "call.heartbeat" {
		return errors.New("unsupported call signal")
	}

	sessionValue, err := a.redisClient.Get(ctx, "call:session:"+callID).Bytes()
	if err != nil {
		return errors.New("call session not found")
	}
	var session callSession
	if err := json.Unmarshal(sessionValue, &session); err != nil {
		return fmt.Errorf("decode call session: %w", err)
	}
	targetID, allowed := session.CalleeID, senderID == session.CallerID
	if senderID == session.CalleeID {
		targetID, allowed = session.CallerID, true
	}
	if !allowed {
		return errors.New("user is not a participant in this call")
	}
	switch event.Type {
	case "call.accept", "call.reject", "call.answer":
		if senderID != session.CalleeID {
			return errors.New("only the callee can send this call signal")
		}
	case "call.offer":
		if senderID != session.CallerID && !readJSONBool(fields["iceRestart"]) {
			return errors.New("only the caller can send an offer")
		}
	}
	if event.Type == "call.heartbeat" {
		if err := a.refreshCallSession(ctx, session); err != nil {
			return err
		}
		return nil
	}
	if event.Type == "call.reject" || event.Type == "call.end" {
		defer a.removeCallSession(session)
	} else if err := a.refreshCallSession(ctx, session); err != nil {
		return err
	}
	if err := a.publishCallEvent(ctx, targetID, event.Type, fields); err != nil {
		return err
	}
	return nil
}

func (a *App) createCallSession(ctx context.Context, callerID, callID string, fields map[string]json.RawMessage) error {
	conversationID := readJSONText(fields["conversationId"])
	callType := readJSONText(fields["callType"])
	if _, err := uuid.Parse(conversationID); err != nil {
		return errors.New("invalid conversation id")
	}
	if callType != "audio" && callType != "video" {
		return errors.New("call type must be audio or video")
	}
	conversations, err := a.services.ListConversations(ctx, callerID)
	if err != nil {
		return fmt.Errorf("validate call conversation: %w", err)
	}
	var calleeID string
	for _, conversation := range conversations {
		if conversation.ID == conversationID {
			calleeID = conversation.Participant.ID
			break
		}
	}
	if calleeID == "" || calleeID == callerID {
		return errors.New("conversation participant not found")
	}
	isFriend, err := a.services.IsFriend(ctx, callerID, calleeID)
	if err != nil {
		return fmt.Errorf("validate call friendship: %w", err)
	}
	if !isFriend {
		return errors.New("calls are only allowed between friends")
	}
	online, err := a.redisClient.Exists(ctx, "presence:user:"+calleeID).Result()
	if err != nil {
		return fmt.Errorf("check call recipient presence: %w", err)
	}
	if online == 0 {
		return errors.New("call recipient is offline")
	}

	session := callSession{
		CallID: callID, ConversationID: conversationID,
		CallerID: callerID, CalleeID: calleeID, CallType: callType,
	}
	callerKey, calleeKey := "call:user:active:"+callerID, "call:user:active:"+calleeID
	acquiredCaller, err := a.redisClient.SetNX(ctx, callerKey, callID, callSessionTTL).Result()
	if err != nil {
		return fmt.Errorf("reserve caller for call: %w", err)
	}
	if !acquiredCaller {
		return errors.New("caller is already in another call")
	}
	acquiredCallee, err := a.redisClient.SetNX(ctx, calleeKey, callID, callSessionTTL).Result()
	if err != nil || !acquiredCallee {
		_ = a.redisClient.Del(ctx, callerKey).Err()
		if err != nil {
			return fmt.Errorf("reserve call recipient: %w", err)
		}
		payload := map[string]json.RawMessage{}
		payload["callId"], _ = json.Marshal(callID)
		payload["reason"], _ = json.Marshal("busy")
		if publishErr := a.publishCallEvent(ctx, callerID, "call.reject", payload); publishErr != nil {
			return publishErr
		}
		return nil
	}
	sessionValue, err := json.Marshal(session)
	if err != nil {
		_ = a.redisClient.Del(ctx, callerKey, calleeKey).Err()
		return fmt.Errorf("encode call session: %w", err)
	}
	if err := a.redisClient.Set(ctx, "call:session:"+callID, sessionValue, callSessionTTL).Err(); err != nil {
		_ = a.redisClient.Del(ctx, callerKey, calleeKey).Err()
		return fmt.Errorf("save call session: %w", err)
	}
	fields["from"], _ = json.Marshal(callerID)
	if err := a.publishCallEvent(ctx, calleeID, "call.invite", fields); err != nil {
		a.removeCallSession(session)
		return err
	}
	return nil
}

func (a *App) refreshCallSession(ctx context.Context, session callSession) error {
	sessionValue, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode call session: %w", err)
	}
	pipe := a.redisClient.TxPipeline()
	pipe.Set(ctx, "call:session:"+session.CallID, sessionValue, callSessionTTL)
	pipe.Set(ctx, "call:user:active:"+session.CallerID, session.CallID, callSessionTTL)
	pipe.Set(ctx, "call:user:active:"+session.CalleeID, session.CallID, callSessionTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("refresh call session: %w", err)
	}
	return nil
}

func (a *App) removeCallSession(session callSession) {
	if a.redisClient == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = a.redisClient.Del(
		ctx,
		"call:session:"+session.CallID,
		"call:user:active:"+session.CallerID,
		"call:user:active:"+session.CalleeID,
	).Err()
}

func (a *App) publishCallEvent(ctx context.Context, recipientID, eventType string, payload map[string]json.RawMessage) error {
	data, err := json.Marshal(map[string]any{
		"type":    eventType,
		"payload": payload,
		"ts":      time.Now().UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("encode call signal: %w", err)
	}
	if err := a.redisClient.Publish(ctx, "call:user:"+recipientID, data).Err(); err != nil {
		return fmt.Errorf("relay call signal: %w", err)
	}
	return nil
}

func readJSONText(value json.RawMessage) string {
	var result string
	_ = json.Unmarshal(value, &result)
	return strings.TrimSpace(result)
}

func readJSONBool(value json.RawMessage) bool {
	var result bool
	_ = json.Unmarshal(value, &result)
	return result
}
