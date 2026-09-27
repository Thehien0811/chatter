package auth

import (
	"testing"
	"time"
)

func TestJWTManager_GenerateAndValidate(t *testing.T) {
	manager := NewJWTManager("super-secret-key")
	token, err := manager.Generate("user-123", "user", 30*time.Minute)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}

	if claims.UserID != "user-123" {
		t.Fatalf("expected user id user-123, got %s", claims.UserID)
	}

	if claims.Role != "user" {
		t.Fatalf("expected role user, got %s", claims.Role)
	}
}

func TestCasbinEnforcer_AllowRoleBasedAccess(t *testing.T) {
	enforcer := NewCasbinEnforcer()

	if ok, err := enforcer.Enforce("user", "/api/v1/users/:id", "GET"); err != nil || !ok {
		t.Fatalf("expected user to read own profile, got ok=%v err=%v", ok, err)
	}

	if ok, err := enforcer.Enforce("user", "/api/v1/admin/users", "GET"); err != nil || ok {
		t.Fatalf("expected user to be denied admin access, got ok=%v err=%v", ok, err)
	}

	if ok, err := enforcer.Enforce("admin", "/api/v1/admin/users", "GET"); err != nil || !ok {
		t.Fatalf("expected admin to access admin users, got ok=%v err=%v", ok, err)
	}
}
