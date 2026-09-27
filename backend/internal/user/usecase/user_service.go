package usecase

import "github.com/thehien0811/chatter/internal/user/domain"

// UserService contains business logic for the user domain.
type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) GetByID(id string) *domain.User {
	return &domain.User{ID: id, Username: "demo_user", Email: "demo@example.com", Presence: "online"}
}
