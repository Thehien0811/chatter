package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/thehien0811/chatter/internal/user/domain"
	"github.com/thehien0811/chatter/internal/user/repository"
	"golang.org/x/crypto/bcrypt"
)

// UserUseCase contains the core user business logic for signup/login/profile.
type UserUseCase struct {
	repo *repository.UserRepository
}

func NewUserUseCase(repo *repository.UserRepository) *UserUseCase {
	return &UserUseCase{repo: repo}
}

func (u *UserUseCase) Register(ctx context.Context, username, displayName, email, password string) (*domain.User, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return nil, errors.New("username and password are required")
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = username
	}

	exists, err := u.repo.ExistsByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("username already exists")
	}

	if email != "" {
		exists, err = u.repo.ExistsByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("email already exists")
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Username:      username,
		DisplayName:   displayName,
		Email:         email,
		PasswordHash:  string(hash),
		Presence:      "offline",
		StatusMessage: "",
	}

	if err := u.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (u *UserUseCase) Authenticate(ctx context.Context, username, password string) (*domain.User, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return nil, errors.New("username and password are required")
	}

	user, err := u.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	return user, nil
}

func (u *UserUseCase) GetProfile(ctx context.Context, userID string) (*domain.User, error) {
	user, err := u.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (u *UserUseCase) UpdateProfile(ctx context.Context, userID, displayName, email, statusMessage string) (*domain.User, error) {
	user, err := u.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	if strings.TrimSpace(displayName) != "" {
		user.DisplayName = displayName
	}
	if strings.TrimSpace(email) != "" {
		user.Email = email
	}
	if statusMessage != "" {
		user.StatusMessage = statusMessage
	}
	user.UpdatedAt = time.Now()

	if err := u.repo.UpdateProfile(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}
