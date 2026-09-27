package domain

import jwt "github.com/golang-jwt/jwt/v5"

// Claims is the canonical token payload used across services.
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}
