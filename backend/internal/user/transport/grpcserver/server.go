package grpcserver

import (
	"context"
	"strings"

	"github.com/thehien0811/chatter/internal/user/domain"
	"github.com/thehien0811/chatter/internal/user/usecase"
	pb "github.com/thehien0811/chatter/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedUserServiceServer
	users *usecase.UserUseCase
}

func New(users *usecase.UserUseCase) *Server {
	return &Server{users: users}
}

func (s *Server) Register(ctx context.Context, req *pb.RegisterUserRequest) (*pb.User, error) {
	if req.GetUsername() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password are required")
	}
	user, err := s.users.Register(ctx, req.GetUsername(), req.GetDisplayName(), req.GetEmail(), req.GetPassword())
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		if strings.Contains(err.Error(), "required") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "could not register user")
	}
	return toProto(user), nil
}

func (s *Server) GetUserByID(ctx context.Context, req *pb.GetUserByIDRequest) (*pb.User, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	user, err := s.users.GetProfile(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return toProto(user), nil
}

func (s *Server) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.User, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	user, err := s.users.UpdateProfile(ctx, req.GetUserId(), req.GetDisplayName(), req.GetEmail(), req.GetStatusMessage())
	if err != nil {
		if err.Error() == "user not found" {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "could not update user")
	}
	return toProto(user), nil
}

func (s *Server) ValidateCredentials(ctx context.Context, req *pb.ValidateCredentialsRequest) (*pb.User, error) {
	user, err := s.users.Authenticate(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}
	return toProto(user), nil
}

func toProto(user *domain.User) *pb.User {
	return &pb.User{
		Id: user.ID, Username: user.Username, DisplayName: user.DisplayName,
		Email: user.Email, Presence: user.Presence, StatusMessage: user.StatusMessage,
	}
}
