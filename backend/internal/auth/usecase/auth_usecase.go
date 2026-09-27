package usecase

import (
	"time"

	"github.com/thehien0811/chatter/internal/auth/app"
)

// AuthUseCase is the application use case for auth operations.
type AuthUseCase struct {
	jwt  *app.JWTService
	casb *app.CasbinService
}

func NewAuthUseCase(jwtService *app.JWTService, casbinService *app.CasbinService) *AuthUseCase {
	return &AuthUseCase{jwt: jwtService, casb: casbinService}
}

func (u *AuthUseCase) Login(username, password string) (string, string, error) {
	role := "user"
	if username == "admin" {
		role = "admin"
	}

	token, err := u.jwt.Generate("user-"+username, role, 2*time.Hour)
	if err != nil {
		return "", "", err
	}

	return token, role, nil
}

func (u *AuthUseCase) Authorize(role, obj, method string) (bool, error) {
	return u.casb.Enforce(role, obj, method)
}
