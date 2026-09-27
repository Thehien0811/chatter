package app

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRefreshTokenManager_StoreAndValidate(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	manager := NewRefreshTokenManager(client, "secret-key", 7*24*time.Hour)

	token, err := manager.Issue("user-123")
	if err != nil {
		t.Fatalf("issue refresh token: %v", err)
	}

	userID, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("validate refresh token: %v", err)
	}
	if userID != "user-123" {
		t.Fatalf("expected user-123, got %s", userID)
	}

	newToken, err := manager.Rotate(token)
	if err != nil {
		t.Fatalf("rotate refresh token: %v", err)
	}
	if newToken == token {
		t.Fatal("rotated token should be different from original")
	}

	if _, err := manager.Validate(token); err == nil {
		t.Fatal("old refresh token should be invalid after rotation")
	}
}
