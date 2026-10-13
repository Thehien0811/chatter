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
	authConn   *grpc.ClientConn
	userConn   *grpc.ClientConn
	chatConn   *grpc.ClientConn
	friendConn *grpc.ClientConn

	auth   pb.AuthServiceClient
	user   pb.UserServiceClient
	chat   pb.ChatServiceClient
	friend pb.FriendServiceClient
}

type RemoteUser struct {
	ID, Username, DisplayName, Email, Presence, StatusMessage string
}

type AuthTokens struct {
	UserID, AccessToken, RefreshToken string
}

type ChatParticipant struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Presence    string `json:"presence"`
}

type ChatMessage struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	Body           string `json:"body"`
	CreatedAt      string `json:"created_at"`
}

type ChatConversation struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Participant ChatParticipant `json:"participant"`
	LastMessage *ChatMessage    `json:"last_message,omitempty"`
	UpdatedAt   string          `json:"updated_at"`
	UnreadCount int32           `json:"unread_count"`
}

type FriendUser struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type FriendRequest struct {
	ID        string     `json:"id"`
	FromUser  FriendUser `json:"from_user"`
	ToUser    FriendUser `json:"to_user"`
	Status    string     `json:"status"`
	CreatedAt string     `json:"created_at"`
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
	friendConn, err := dialService(cfg.Services.FriendURL)
	if err != nil {
		_ = authConn.Close()
		_ = userConn.Close()
		_ = chatConn.Close()
		return nil, fmt.Errorf("dial friend service: %w", err)
	}
	return &ServiceClients{
		authConn:   authConn,
		userConn:   userConn,
		chatConn:   chatConn,
		friendConn: friendConn,
		auth:       pb.NewAuthServiceClient(authConn),
		user:       pb.NewUserServiceClient(userConn),
		chat:       pb.NewChatServiceClient(chatConn),
		friend:     pb.NewFriendServiceClient(friendConn),
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
	if s.friendConn != nil {
		_ = s.friendConn.Close()
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

func (s *ServiceClients) GetOrCreateDirectConversation(ctx context.Context, userID, recipientID string) (*ChatConversation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	item, err := s.chat.GetOrCreateDirectConversation(ctx, &pb.GetOrCreateDirectConversationRequest{UserId: userID, RecipientId: recipientID})
	if err != nil {
		return nil, err
	}
	return chatConversation(item), nil
}

func (s *ServiceClients) ListConversations(ctx context.Context, userID string) ([]ChatConversation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := s.chat.ListConversations(ctx, &pb.ListConversationsRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	items := make([]ChatConversation, 0, len(response.GetConversations()))
	for _, item := range response.GetConversations() {
		items = append(items, *chatConversation(item))
	}
	return items, nil
}

func (s *ServiceClients) ListMessages(ctx context.Context, conversationID, userID string, limit int32) ([]ChatMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := s.chat.ListMessages(ctx, &pb.ListMessagesRequest{ConversationId: conversationID, UserId: userID, Limit: limit})
	if err != nil {
		return nil, err
	}
	items := make([]ChatMessage, 0, len(response.GetMessages()))
	for _, m := range response.GetMessages() {
		items = append(items, ChatMessage{ID: m.GetId(), ConversationID: m.GetConversationId(), SenderID: m.GetSenderId(), Body: m.GetBody(), CreatedAt: m.GetCreatedAt()})
	}
	return items, nil
}

func chatConversation(item *pb.Conversation) *ChatConversation {
	result := &ChatConversation{ID: item.GetId(), Title: item.GetTitle(), Participant: ChatParticipant{ID: item.GetParticipantId(), Username: item.GetParticipantUsername(), DisplayName: item.GetParticipantDisplayName(), Presence: item.GetParticipantPresence()}, UpdatedAt: item.GetUpdatedAt(), UnreadCount: item.GetUnreadCount()}
	if item.GetLastMessageId() != "" {
		result.LastMessage = &ChatMessage{ID: item.GetLastMessageId(), ConversationID: item.GetId(), SenderID: item.GetLastMessageSenderId(), Body: item.GetLastMessageBody(), CreatedAt: item.GetUpdatedAt()}
	}
	return result
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

func (s *ServiceClients) SearchUsers(ctx context.Context, userID, query string) ([]FriendUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := s.friend.SearchUsers(ctx, &pb.SearchUsersRequest{Query: query, ExcludeUserId: userID, Limit: 20})
	if err != nil {
		return nil, err
	}
	users := make([]FriendUser, 0, len(response.GetUsers()))
	for _, user := range response.GetUsers() {
		users = append(users, friendUser(user))
	}
	return users, nil
}

func (s *ServiceClients) SendFriendRequest(ctx context.Context, fromUserID, toUserID string) (*FriendRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := s.friend.SendFriendRequest(ctx, &pb.SendFriendRequestRequest{FromUserId: fromUserID, ToUserId: toUserID})
	if err != nil {
		return nil, err
	}
	return friendRequest(request), nil
}

func (s *ServiceClients) ListFriendRequests(ctx context.Context, userID, direction string) ([]FriendRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := s.friend.ListFriendRequests(ctx, &pb.ListFriendRequestsRequest{UserId: userID, Direction: direction})
	if err != nil {
		return nil, err
	}
	requests := make([]FriendRequest, 0, len(response.GetRequests()))
	for _, request := range response.GetRequests() {
		requests = append(requests, *friendRequest(request))
	}
	return requests, nil
}

func (s *ServiceClients) RespondFriendRequest(ctx context.Context, requestID, userID string, accept bool) (*FriendRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := s.friend.RespondFriendRequest(ctx, &pb.RespondFriendRequestRequest{RequestId: requestID, UserId: userID, Accept: accept})
	if err != nil {
		return nil, err
	}
	return friendRequest(request), nil
}

func (s *ServiceClients) ListFriends(ctx context.Context, userID string) ([]FriendUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := s.friend.ListFriends(ctx, &pb.ListFriendsRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	friends := make([]FriendUser, 0, len(response.GetFriends()))
	for _, user := range response.GetFriends() {
		friends = append(friends, friendUser(user))
	}
	return friends, nil
}

func friendUser(user *pb.FriendUser) FriendUser {
	return FriendUser{ID: user.GetId(), Username: user.GetUsername(), DisplayName: user.GetDisplayName()}
}

func friendRequest(request *pb.FriendRequest) *FriendRequest {
	return &FriendRequest{ID: request.GetId(), FromUser: friendUser(request.GetFromUser()), ToUser: friendUser(request.GetToUser()), Status: request.GetStatus(), CreatedAt: request.GetCreatedAt()}
}

func remoteUser(user *pb.User) *RemoteUser {
	return &RemoteUser{
		ID: user.GetId(), Username: user.GetUsername(), DisplayName: user.GetDisplayName(),
		Email: user.GetEmail(), Presence: user.GetPresence(), StatusMessage: user.GetStatusMessage(),
	}
}
