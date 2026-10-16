package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestCallSignalRelaysOnlyToCallParticipant(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	session := callSession{CallID: "00000000-0000-4000-8000-000000000001", ConversationID: "conversation-1", CallerID: "caller-1", CalleeID: "callee-1", CallType: "audio"}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, "call:session:"+session.CallID, sessionJSON, callSessionTTL).Err(); err != nil {
		t.Fatal(err)
	}
	subscription := client.Subscribe(ctx, "call:user:callee-1")
	t.Cleanup(func() { _ = subscription.Close() })
	if _, err := subscription.Receive(ctx); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"type":    "call.offer",
		"payload": map[string]any{"callId": session.CallID, "sdp": map[string]string{"type": "offer", "sdp": "test-sdp"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	gateway := &App{redisClient: client, services: &ServiceClients{}}
	if err := gateway.handleCallSignal(ctx, session.CallerID, payload); err != nil {
		t.Fatalf("handleCallSignal() error = %v", err)
	}
	message, err := subscription.ReceiveMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var relayed struct {
		Type    string `json:"type"`
		Payload struct {
			CallID string `json:"callId"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(message.Payload), &relayed); err != nil {
		t.Fatal(err)
	}
	if relayed.Type != "call.offer" || relayed.Payload.CallID != session.CallID {
		t.Fatalf("relayed event = %#v, want call.offer for %s", relayed, session.CallID)
	}
}

func TestCallSignalRejectsNonParticipant(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()

	session := callSession{CallID: "00000000-0000-4000-8000-000000000002", ConversationID: "conversation-2", CallerID: "caller-2", CalleeID: "callee-2", CallType: "audio"}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, "call:session:"+session.CallID, sessionJSON, callSessionTTL).Err(); err != nil {
		t.Fatal(err)
	}
	gateway := &App{redisClient: client, services: &ServiceClients{}}
	payload, err := json.Marshal(map[string]any{
		"type":    "call.ice",
		"payload": map[string]any{"callId": session.CallID, "candidate": map[string]string{"candidate": "candidate"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := gateway.handleCallSignal(ctx, "unrelated-user", payload); err == nil {
		t.Fatal("handleCallSignal() accepted a non-participant")
	}
}

func TestCallEndReleasesCallReservations(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	session := callSession{CallID: "00000000-0000-4000-8000-000000000003", ConversationID: "conversation-3", CallerID: "caller-3", CalleeID: "callee-3", CallType: "audio"}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"call:session:" + session.CallID:       string(sessionJSON),
		"call:user:active:" + session.CallerID: session.CallID,
		"call:user:active:" + session.CalleeID: session.CallID,
	} {
		if err := client.Set(ctx, key, value, callSessionTTL).Err(); err != nil {
			t.Fatal(err)
		}
	}
	subscription := client.Subscribe(ctx, "call:user:callee-3")
	t.Cleanup(func() { _ = subscription.Close() })
	if _, err := subscription.Receive(ctx); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"type":    "call.end",
		"payload": map[string]string{"callId": session.CallID, "reason": "hangup"},
	})
	if err != nil {
		t.Fatal(err)
	}
	gateway := &App{redisClient: client, services: &ServiceClients{}}
	if err := gateway.handleCallSignal(ctx, session.CallerID, payload); err != nil {
		t.Fatalf("handleCallSignal() error = %v", err)
	}
	if client.Exists(ctx, "call:session:"+session.CallID, "call:user:active:"+session.CallerID, "call:user:active:"+session.CalleeID).Val() != 0 {
		t.Fatal("call session and reservations should be removed")
	}
}
