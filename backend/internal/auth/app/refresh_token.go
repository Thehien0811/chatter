package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// RefreshTokenManager handles refresh token issuance and rotation using Redis.
type RefreshTokenManager struct {
	client *redis.Client
	secret string
	ttl    time.Duration
}

func NewRefreshTokenManager(client *redis.Client, secret string, ttl time.Duration) *RefreshTokenManager {
	return &RefreshTokenManager{client: client, secret: secret, ttl: ttl}
}

func (m *RefreshTokenManager) Issue(userID string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	token := base64.RawURLEncoding.EncodeToString(bytes)
	claims := jwt.MapClaims{
		"sub": userID,
		"jti": token,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(m.ttl).Unix(),
	}

	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(m.secret))
	if err != nil {
		return "", err
	}

	if err := m.client.Set(context.Background(), "refresh:"+token, userID, m.ttl).Err(); err != nil {
		return "", err
	}

	return tokenString, nil
}

func (m *RefreshTokenManager) Validate(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secret), nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid refresh token")
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return "", errors.New("invalid refresh token payload")
	}

	jti, _ := claims["jti"].(string)
	if jti == "" {
		return "", errors.New("invalid refresh token identifier")
	}

	stored, err := m.client.Get(context.Background(), "refresh:"+jti).Result()
	if err != nil || stored == "" {
		return "", errors.New("refresh token not found or revoked")
	}
	if stored != userID {
		return "", errors.New("refresh token mismatch")
	}

	return userID, nil
}

func (m *RefreshTokenManager) Rotate(tokenString string) (string, error) {
	userID, err := m.Validate(tokenString)
	if err != nil {
		return "", err
	}

	if err := m.Revoke(tokenString); err != nil {
		return "", err
	}

	return m.Issue(userID)
}

func (m *RefreshTokenManager) Revoke(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secret), nil
	})
	if err != nil {
		return err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return errors.New("invalid refresh token")
	}

	jti, _ := claims["jti"].(string)
	if jti == "" {
		return errors.New("invalid refresh token identifier")
	}

	return m.client.Del(context.Background(), "refresh:"+jti).Err()
}
