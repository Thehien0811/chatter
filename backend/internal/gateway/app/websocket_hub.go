package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var errSubscriptionTimeout = errors.New("timed out subscribing to user events")

// websocketHub maintains this gateway instance's user-to-connection pool and
// one Redis subscription for each locally connected user.
type websocketHub struct {
	redis  *redis.Client
	mu     sync.RWMutex
	users  map[string]map[*websocket.Conn]*sync.Mutex
	cancel map[string]context.CancelFunc
}

func newWebsocketHub(client *redis.Client) *websocketHub {
	return &websocketHub{
		redis:  client,
		users:  make(map[string]map[*websocket.Conn]*sync.Mutex),
		cancel: make(map[string]context.CancelFunc),
	}
}

func (h *websocketHub) register(userID string, conn *websocket.Conn) (func(), error) {
	writeMu := &sync.Mutex{}
	h.mu.Lock()
	conns := h.users[userID]
	first := len(conns) == 0
	if conns == nil {
		conns = make(map[*websocket.Conn]*sync.Mutex)
		h.users[userID] = conns
	}
	conns[conn] = writeMu

	var ready chan error
	if first && h.redis != nil {
		ctx, cancel := context.WithCancel(context.Background())
		h.cancel[userID] = cancel
		ready = make(chan error, 1)
		go h.subscribe(ctx, userID, ready)
	}
	h.mu.Unlock()

	var once sync.Once
	unregister := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.users[userID], conn)
			if len(h.users[userID]) == 0 {
				delete(h.users, userID)
				if cancel := h.cancel[userID]; cancel != nil {
					cancel()
					delete(h.cancel, userID)
				}
			}
			h.mu.Unlock()
		})
	}

	if ready != nil {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case err := <-ready:
			if err != nil {
				unregister()
				return nil, err
			}
		case <-timer.C:
			unregister()
			return nil, errSubscriptionTimeout
		}
	}
	return unregister, nil
}

func (h *websocketHub) subscribe(ctx context.Context, userID string, ready chan<- error) {
	pubsub := h.redis.Subscribe(ctx, "chat:user:"+userID, "friend:user:"+userID)
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		ready <- err
		return
	}
	ready <- nil
	channel := pubsub.Channel()
	for {
		select {
		case event, ok := <-channel:
			if !ok {
				return
			}
			h.broadcast(userID, []byte(event.Payload))
		case <-ctx.Done():
			return
		}
	}
}

func (h *websocketHub) hasUser(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.users[userID]) > 0
}

func (h *websocketHub) writeConnection(conn *websocket.Conn, payload []byte) error {
	h.mu.RLock()
	var userID string
	var writeMu *sync.Mutex
	for id, conns := range h.users {
		if candidate, ok := conns[conn]; ok {
			userID, writeMu = id, candidate
			break
		}
	}
	h.mu.RUnlock()
	if writeMu == nil {
		return websocket.ErrCloseSent
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	h.mu.RLock()
	active := h.users[userID][conn] == writeMu
	h.mu.RUnlock()
	if !active {
		return websocket.ErrCloseSent
	}
	return conn.WriteMessage(websocket.TextMessage, payload)
}

func (h *websocketHub) broadcast(userID string, payload []byte) {
	h.mu.RLock()
	conns := make(map[*websocket.Conn]*sync.Mutex, len(h.users[userID]))
	for conn, writeMu := range h.users[userID] {
		conns[conn] = writeMu
	}
	h.mu.RUnlock()

	for conn, writeMu := range conns {
		writeMu.Lock()
		h.mu.RLock()
		active := h.users[userID][conn] == writeMu
		h.mu.RUnlock()
		var err error
		if active {
			err = conn.WriteMessage(websocket.TextMessage, payload)
		}
		writeMu.Unlock()
		if active && err != nil {
			h.mu.Lock()
			delete(h.users[userID], conn)
			if len(h.users[userID]) == 0 {
				delete(h.users, userID)
				if cancel := h.cancel[userID]; cancel != nil {
					cancel()
					delete(h.cancel, userID)
				}
			}
			h.mu.Unlock()
			_ = conn.Close()
		}
	}
}
