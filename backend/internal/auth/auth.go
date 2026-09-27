package auth

import (
	"time"

	"github.com/thehien0811/chatter/internal/auth/app"
	"github.com/thehien0811/chatter/internal/auth/domain"
)

// Claims is kept for compatibility with older code.
type Claims = domain.Claims

// JWTManager wraps the JWT service implementation used by the app layer.
type JWTManager struct {
	svc *app.JWTService
}

func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{svc: app.NewJWTService(secret)}
}

func (m *JWTManager) Generate(userID, role string, ttl time.Duration) (string, error) {
	return m.svc.Generate(userID, role, ttl)
}

func (m *JWTManager) Validate(tokenString string) (*domain.Claims, error) {
	return m.svc.Validate(tokenString)
}

// CasbinEnforcer wraps the Casbin service implementation.
type CasbinEnforcer struct {
	svc *app.CasbinService
}

func NewCasbinEnforcer() *CasbinEnforcer {
	return &CasbinEnforcer{svc: app.NewCasbinService()}
}

func (c *CasbinEnforcer) Enforce(sub, obj, act string) (bool, error) {
	return c.svc.Enforce(sub, obj, act)
}
