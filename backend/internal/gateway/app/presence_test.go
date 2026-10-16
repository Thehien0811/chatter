package app

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestPresenceTracksMultipleConnections(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	gateway := &App{redisClient: client}

	firstVersion, err := gateway.updatePresenceConnection(ctx, "user-1", "connection-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if firstVersion == 0 {
		t.Fatal("first connection should transition the user online")
	}

	secondVersion, err := gateway.updatePresenceConnection(ctx, "user-1", "connection-2", true)
	if err != nil {
		t.Fatal(err)
	}
	if secondVersion != 0 {
		t.Fatalf("second connection should not emit another online transition, got version %d", secondVersion)
	}

	closedFirstVersion, err := gateway.updatePresenceConnection(ctx, "user-1", "connection-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if closedFirstVersion != 0 {
		t.Fatalf("closing one of two connections should not transition offline, got version %d", closedFirstVersion)
	}
	snapshot, err := gateway.getUserPresence(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != "online" {
		t.Fatalf("expected remaining connection to keep user online, got %q", snapshot.Status)
	}

	closedLastVersion, err := gateway.updatePresenceConnection(ctx, "user-1", "connection-2", false)
	if err != nil {
		t.Fatal(err)
	}
	if closedLastVersion <= firstVersion {
		t.Fatalf("last connection should increment presence version, got %d after %d", closedLastVersion, firstVersion)
	}
	snapshot, err = gateway.getUserPresence(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != "offline" {
		t.Fatalf("expected user to be offline after the last connection closes, got %q", snapshot.Status)
	}
}

func TestPresenceLeaseExpiresWithoutHeartbeat(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	gateway := &App{redisClient: client}

	if _, err := gateway.updatePresenceConnection(ctx, "user-2", "connection-1", true); err != nil {
		t.Fatal(err)
	}
	server.FastForward(presenceLeaseTTL + time.Millisecond)

	snapshot, err := gateway.getUserPresence(ctx, "user-2")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != "offline" {
		t.Fatalf("expected expired lease to report offline, got %q", snapshot.Status)
	}
}
