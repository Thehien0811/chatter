package app

import (
	"errors"
	"fmt"
	"time"

	casbin "github.com/casbin/casbin/v2"
	casbinModelPkg "github.com/casbin/casbin/v2/model"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/thehien0811/chatter/internal/auth/domain"
)

// JWTService issues and validates JWT tokens.
type JWTService struct {
	secret []byte
}

func NewJWTService(secret string) *JWTService {
	return &JWTService{secret: []byte(secret)}
}

func (s *JWTService) Generate(userID, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := domain.Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *JWTService) Validate(tokenString string) (*domain.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &domain.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*domain.Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// CasbinService wraps the policy engine with application policies.
type CasbinService struct {
	*casbin.Enforcer
}

const casbinModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (g(r.sub, p.sub) || r.sub == p.sub) && keyMatch2(r.obj, p.obj) && r.act == p.act
`

func NewCasbinService() *CasbinService {
	model, err := casbinModelPkg.NewModelFromString(casbinModel)
	if err != nil {
		panic(err)
	}

	enforcer, err := casbin.NewEnforcer(model)
	if err != nil {
		panic(err)
	}

	_, _ = enforcer.AddPolicy("user", "/api/v1/profile", "GET")
	_, _ = enforcer.AddPolicy("user", "/api/v1/users/:id", "GET")
	_, _ = enforcer.AddPolicy("user", "/api/v1/users/:id", "PATCH")
	_, _ = enforcer.AddPolicy("user", "/api/v1/chat/unread", "GET")
	_, _ = enforcer.AddPolicy("user", "/api/v1/friends", "GET")
	_, _ = enforcer.AddPolicy("user", "/api/v1/friend-requests", "GET")
	_, _ = enforcer.AddPolicy("user", "/api/v1/friend-requests", "POST")
	_, _ = enforcer.AddPolicy("user", "/api/v1/friend-requests/:id/accept", "POST")
	_, _ = enforcer.AddPolicy("user", "/api/v1/friend-requests/:id/reject", "POST")
	_, _ = enforcer.AddPolicy("user", "/api/v1/users/search", "GET")
	_, _ = enforcer.AddPolicy("user", "/api/v1/chat/conversations", "GET")
	_, _ = enforcer.AddPolicy("user", "/api/v1/chat/conversations", "POST")
	_, _ = enforcer.AddPolicy("user", "/api/v1/chat/conversations/:id/messages", "GET")
	_, _ = enforcer.AddPolicy("user", "/api/v1/chat/messages/:id/delivered", "POST")
	_, _ = enforcer.AddPolicy("user", "/api/v1/ws/presence", "GET")
	_, _ = enforcer.AddPolicy("admin", "/api/v1/admin/*", "GET")
	_, _ = enforcer.AddPolicy("admin", "/api/v1/admin/*", "POST")
	_, _ = enforcer.AddPolicy("admin", "/api/v1/admin/*", "PATCH")
	_, _ = enforcer.AddPolicy("admin", "/api/v1/admin/*", "DELETE")

	return &CasbinService{Enforcer: enforcer}
}

func (s *CasbinService) Enforce(sub, obj, act string) (bool, error) {
	return s.Enforcer.Enforce(sub, obj, act)
}
