package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/thehien0811/chatter/config"
	pb "github.com/thehien0811/chatter/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ServiceClients struct {
	authConn *grpc.ClientConn
	userConn *grpc.ClientConn
	chatConn *grpc.ClientConn

	auth pb.AuthServiceClient
	user pb.UserServiceClient
	chat pb.ChatServiceClient
}

type RemoteUser struct {
	ID, Username, DisplayName, Email, Presence, StatusMessage string
}

type AuthTokens struct {
	UserID, AccessToken, RefreshToken string
}

type UnreadMessage struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	Body           string `json:"body"`
	CreatedAt      string `json:"created_at"`
}

func NewServiceClients(cfg *config.AppConfig) (*ServiceClients, error) {
	if cfg == nil {
		return nil, fmt.Errorf("service configuration is required")
	}
	authConn, err := dialService(cfg.Services.AuthURL)
	if err != nil {
		return nil, fmt.Errorf("dial auth service: %w", err)
	}
	userConn, err := dialService(cfg.Services.UserURL)
	if err != nil {
		_ = authConn.Close()
		return nil, fmt.Errorf("dial user service: %w", err)
	}
	chatConn, err := dialService(cfg.Services.ChatURL)
	if err != nil {
		_ = authConn.Close()
		_ = userConn.Close()
		return nil, fmt.Errorf("dial chat service: %w", err)
	}
	return &ServiceClients{
		authConn: authConn,
		userConn: userConn,
		chatConn: chatConn,
		auth:     pb.NewAuthServiceClient(authConn),
		user:     pb.NewUserServiceClient(userConn),
		chat:     pb.NewChatServiceClient(chatConn),
	}, nil
}

func dialService(target string) (*grpc.ClientConn, error) {
	target = strings.TrimPrefix(strings.TrimPrefix(target, "http://"), "https://")
	if target == "" {
		return nil, fmt.Errorf("empty gRPC target")
	}
	return grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func (s *ServiceClients) Close() {
	if s == nil {
		return
	}
	if s.authConn != nil {
		_ = s.authConn.Close()
	}
	if s.userConn != nil {
		_ = s.userConn.Close()
	}
	if s.chatConn != nil {
		_ = s.chatConn.Close()
	}
}

func (s *ServiceClients) Register(ctx context.Context, username, displayName, email, password string) (*RemoteUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	user, err := s.user.Register(ctx, &pb.RegisterUserRequest{
		Username: username, DisplayName: displayName, Email: email, Password: password,
	})
	if err != nil {
		return nil, err
	}
	return remoteUser(user), nil
}

func (s *ServiceClients) Login(ctx context.Context, username, password string) (*AuthTokens, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tokens, err := s.auth.Login(ctx, &pb.LoginRequest{Username: username, Password: password})
	if err != nil {
		return nil, err
	}
	return &AuthTokens{UserID: tokens.GetUserId(), AccessToken: tokens.GetAccessToken(), RefreshToken: tokens.GetRefreshToken()}, nil
}

func (s *ServiceClients) Refresh(ctx context.Context, refreshToken string) (*AuthTokens, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tokens, err := s.auth.Refresh(ctx, &pb.RefreshRequest{RefreshToken: refreshToken})
	if err != nil {
		return nil, err
	}
	return &AuthTokens{AccessToken: tokens.GetAccessToken(), RefreshToken: tokens.GetRefreshToken()}, nil
}

func (s *ServiceClients) GetUser(ctx context.Context, userID string) (*RemoteUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	user, err := s.user.GetUserByID(ctx, &pb.GetUserByIDRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	return remoteUser(user), nil
}

func (s *ServiceClients) UpdateUser(ctx context.Context, userID, displayName, email, statusMessage string) (*RemoteUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	user, err := s.user.UpdateUser(ctx, &pb.UpdateUserRequest{
		UserId: userID, DisplayName: displayName, Email: email, StatusMessage: statusMessage,
	})
	if err != nil {
		return nil, err
	}
	return remoteUser(user), nil
}

func (s *ServiceClients) SendMessage(ctx context.Context, conversationID, senderID, body string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := s.chat.SendMessage(ctx, &pb.SendMessageRequest{
		ConversationId: conversationID, SenderId: senderID, Body: body,
	})
	if err != nil {
		return "", err
	}
	if !response.GetSuccess() {
		return "", fmt.Errorf("chat service rejected message")
	}
	return response.GetMessageId(), nil
}

func (s *ServiceClients) ListUnread(ctx context.Context, userID string, limit int32) ([]UnreadMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := s.chat.ListUnreadMessages(ctx, &pb.ListUnreadMessagesRequest{UserId: userID, Limit: limit})
	if err != nil {
		return nil, err
	}
	messages := make([]UnreadMessage, 0, len(response.GetMessages()))
	for _, message := range response.GetMessages() {
		messages = append(messages, UnreadMessage{
			MessageID: message.GetMessageId(), ConversationID: message.GetConversationId(),
			SenderID: message.GetSenderId(), Body: message.GetBody(), CreatedAt: message.GetCreatedAt(),
		})
	}
	return messages, nil
}

func (s *ServiceClients) MarkDelivered(ctx context.Context, userID, messageID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := s.chat.MarkDelivered(ctx, &pb.MarkDeliveredRequest{UserId: userID, MessageId: messageID})
	if err != nil {
		return err
	}
	if !response.GetSuccess() {
		return fmt.Errorf("chat service rejected delivery update")
	}
	return nil
}

func remoteUser(user *pb.User) *RemoteUser {
	return &RemoteUser{
		ID: user.GetId(), Username: user.GetUsername(), DisplayName: user.GetDisplayName(),
		Email: user.GetEmail(), Presence: user.GetPresence(), StatusMessage: user.GetStatusMessage(),
	}
}
