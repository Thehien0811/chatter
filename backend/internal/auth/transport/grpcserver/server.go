package grpcserver

import (
	"context"
	"time"

	authapp "github.com/thehien0811/chatter/internal/auth/app"
	pb "github.com/thehien0811/chatter/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedAuthServiceServer
	users     pb.UserServiceClient
	jwt       *authapp.JWTService
	refresh   *authapp.RefreshTokenManager
	accessTTL time.Duration
}

func New(users pb.UserServiceClient, jwt *authapp.JWTService, refresh *authapp.RefreshTokenManager, accessTTL time.Duration) *Server {
	if accessTTL <= 0 {
		accessTTL = 2 * time.Hour
	}
	return &Server{users: users, jwt: jwt, refresh: refresh, accessTTL: accessTTL}
}

func (s *Server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.GetUsername() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password are required")
	}
	user, err := s.users.ValidateCredentials(ctx, &pb.ValidateCredentialsRequest{
		Username: req.GetUsername(), Password: req.GetPassword(),
	})
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}
	access, err := s.jwt.Generate(user.GetId(), "user", s.accessTTL)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not create access token")
	}
	refresh, err := s.refresh.Issue(user.GetId())
	if err != nil {
		return nil, status.Error(codes.Internal, "could not create refresh token")
	}
	return &pb.LoginResponse{UserId: user.GetId(), AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Server) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	if req.GetRefreshToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}
	userID, err := s.refresh.Validate(req.GetRefreshToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}
	refresh, err := s.refresh.Rotate(req.GetRefreshToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}
	access, err := s.jwt.Generate(userID, "user", s.accessTTL)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not create access token")
	}
	return &pb.RefreshResponse{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Server) ValidateToken(_ context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	claims, err := s.jwt.Validate(req.GetAccessToken())
	if err != nil {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}
	return &pb.ValidateTokenResponse{UserId: claims.UserID, Role: claims.Role, Valid: true}, nil
}
