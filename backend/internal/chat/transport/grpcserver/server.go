package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	pb "github.com/thehien0811/chatter/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server persists chat data in Postgres and publishes committed message events
// for gateway instances to fan out to their locally connected users.
type Server struct {
	pb.UnimplementedChatServiceServer
	db     *pgxpool.Pool
	redis  *redis.Client
	friend pb.FriendServiceClient
	rabbit *amqp.Channel
}

func New(db *pgxpool.Pool, redisClient *redis.Client, friendClient pb.FriendServiceClient, rabbitChannel *amqp.Channel) *Server {
	return &Server{db: db, redis: redisClient, friend: friendClient, rabbit: rabbitChannel}
}

type messageEvent struct {
	Type    string       `json:"type"`
	Message eventMessage `json:"message"`
}
type eventMessage struct {
	ID             string            `json:"id"`
	ConversationID string            `json:"conversation_id"`
	SenderID       string            `json:"sender_id"`
	Body           string            `json:"body"`
	CreatedAt      string            `json:"created_at"`
	Attachments    []eventAttachment `json:"attachments,omitempty"`
}
type eventAttachment struct {
	ID        string `json:"id"`
	MediaType string `json:"media_type"`
	ByteSize  int64  `json:"byte_size"`
	URL       string `json:"url"`
}

func (s *Server) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	if s.db == nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	conversationID, senderID, body := strings.TrimSpace(req.GetConversationId()), strings.TrimSpace(req.GetSenderId()), strings.TrimSpace(req.GetBody())
	if conversationID == "" || senderID == "" || body == "" {
		return nil, status.Error(codes.InvalidArgument, "conversation_id, sender_id, and body are required")
	}
	if _, err := uuid.Parse(conversationID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid conversation_id")
	}
	if _, err := uuid.Parse(senderID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sender_id")
	}
	if len(body) > 10000 {
		return nil, status.Error(codes.InvalidArgument, "message body exceeds 10000 characters")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	defer tx.Rollback(ctx)

	var senderActive bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL)`, conversationID, senderID).Scan(&senderActive)
	if err != nil {
		return nil, dbError(err)
	}
	if !senderActive {
		return nil, status.Error(codes.PermissionDenied, "sender is not an active conversation member")
	}

	var messageID string
	var createdAt time.Time
	err = tx.QueryRow(ctx, `INSERT INTO messages (conversation_id, sender_id, body, kind) VALUES ($1,$2,$3,'text') RETURNING id, created_at`, conversationID, senderID, body).Scan(&messageID, &createdAt)
	if err != nil {
		return nil, dbError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE conversations SET last_message_at=$1 WHERE id=$2`, createdAt, conversationID)
	if err != nil {
		return nil, dbError(err)
	}

	rows, err := tx.Query(ctx, `SELECT user_id FROM conversation_members WHERE conversation_id=$1 AND left_at IS NULL AND user_id<>$2`, conversationID, senderID)
	if err != nil {
		return nil, dbError(err)
	}
	var recipients []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, dbError(err)
		}
		recipients = append(recipients, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, dbError(err)
	}
	rows.Close()
	for _, recipientID := range recipients {
		if _, err := tx.Exec(ctx, `INSERT INTO message_deliveries (message_id, user_id) VALUES ($1,$2) ON CONFLICT (message_id,user_id) DO NOTHING`, messageID, recipientID); err != nil {
			return nil, dbError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, dbError(err)
	}

	if s.redis != nil && len(recipients) > 0 {
		payload, _ := json.Marshal(messageEvent{Type: "message", Message: eventMessage{ID: messageID, ConversationID: conversationID, SenderID: senderID, Body: body, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano)}})
		for _, recipientID := range recipients {
			// Persistence is authoritative; a Redis outage must not tell the sender
			// that a committed message failed. Offline clients recover via unread RPC.
			_ = s.redis.Publish(ctx, "chat:user:"+recipientID, payload).Err()
		}
	}
	// Delivery rows are always persisted so offline clients can recover messages
	// from Postgres. Queue a notification only when Redis does not show the
	// recipient as online; notification workers can fan this out to push/email.
	for _, recipientID := range recipients {
		if s.isUserOnline(ctx, recipientID) {
			continue
		}
		s.publishOfflineNotification(ctx, recipientID, messageID, conversationID, senderID, body, createdAt)
	}
	return &pb.SendMessageResponse{MessageId: messageID, Success: true}, nil
}

func (s *Server) SendImageMessage(ctx context.Context, req *pb.SendImageMessageRequest) (*pb.SendImageMessageResponse, error) {
	if s.db == nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	conversationID := strings.TrimSpace(req.GetConversationId())
	senderID := strings.TrimSpace(req.GetSenderId())
	mediaID := strings.TrimSpace(req.GetMediaId())
	storageKey := strings.TrimSpace(req.GetStorageKey())
	mediaType := strings.TrimSpace(req.GetMediaType())
	byteSize := req.GetByteSize()
	if _, err := uuid.Parse(conversationID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid conversation_id")
	}
	if _, err := uuid.Parse(senderID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sender_id")
	}
	if _, err := uuid.Parse(mediaID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid media_id")
	}
	extensions := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif"}
	extension, supported := extensions[mediaType]
	if !supported || byteSize < 1 || byteSize > 10*1024*1024 ||
		storageKey != "images/"+senderID+"/"+mediaID+extension {
		return nil, status.Error(codes.InvalidArgument, "invalid image metadata")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	defer tx.Rollback(ctx)
	var senderActive bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL)`, conversationID, senderID).Scan(&senderActive); err != nil {
		return nil, dbError(err)
	}
	if !senderActive {
		return nil, status.Error(codes.PermissionDenied, "sender is not an active conversation member")
	}

	if _, err := tx.Exec(ctx, `INSERT INTO media_objects (id,owner_id,storage_key,media_type,byte_size) VALUES ($1,$2,$3,$4,$5)`, mediaID, senderID, storageKey, mediaType, byteSize); err != nil {
		return nil, dbError(err)
	}
	message := &pb.Message{ConversationId: conversationID, SenderId: senderID}
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO messages (conversation_id,sender_id,kind) VALUES ($1,$2,'image') RETURNING id,created_at`, conversationID, senderID).Scan(&message.Id, &createdAt); err != nil {
		return nil, dbError(err)
	}
	message.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	if _, err := tx.Exec(ctx, `INSERT INTO message_attachments (message_id,media_object_id,position) VALUES ($1,$2,0)`, message.Id, mediaID); err != nil {
		return nil, dbError(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE conversations SET last_message_at=$1 WHERE id=$2`, createdAt, conversationID); err != nil {
		return nil, dbError(err)
	}
	rows, err := tx.Query(ctx, `SELECT user_id FROM conversation_members WHERE conversation_id=$1 AND left_at IS NULL AND user_id<>$2`, conversationID, senderID)
	if err != nil {
		return nil, dbError(err)
	}
	var recipients []string
	for rows.Next() {
		var recipientID string
		if err := rows.Scan(&recipientID); err != nil {
			rows.Close()
			return nil, dbError(err)
		}
		recipients = append(recipients, recipientID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, dbError(err)
	}
	rows.Close()
	for _, recipientID := range recipients {
		if _, err := tx.Exec(ctx, `INSERT INTO message_deliveries (message_id,user_id) VALUES ($1,$2) ON CONFLICT (message_id,user_id) DO NOTHING`, message.Id, recipientID); err != nil {
			return nil, dbError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, dbError(err)
	}

	attachment := &pb.MessageAttachment{Id: mediaID, MediaType: mediaType, ByteSize: byteSize, Url: "/api/v1/media/" + mediaID}
	message.Attachments = []*pb.MessageAttachment{attachment}
	s.publishImageMessage(ctx, message, recipients, createdAt)
	return &pb.SendImageMessageResponse{Message: message}, nil
}

func (s *Server) publishImageMessage(ctx context.Context, message *pb.Message, recipients []string, createdAt time.Time) {
	if len(recipients) == 0 {
		return
	}
	attachments := make([]eventAttachment, 0, len(message.GetAttachments()))
	for _, attachment := range message.GetAttachments() {
		attachments = append(attachments, eventAttachment{ID: attachment.GetId(), MediaType: attachment.GetMediaType(), ByteSize: attachment.GetByteSize(), URL: attachment.GetUrl()})
	}
	payload, err := json.Marshal(messageEvent{Type: "message", Message: eventMessage{
		ID: message.GetId(), ConversationID: message.GetConversationId(), SenderID: message.GetSenderId(),
		CreatedAt: message.GetCreatedAt(), Attachments: attachments,
	}})
	if err != nil {
		log.Printf("encode committed image message event: %v", err)
		return
	}
	for _, recipientID := range recipients {
		if s.redis != nil {
			if err := s.redis.Publish(ctx, "chat:user:"+recipientID, payload).Err(); err != nil {
				log.Printf("publish image message event: %v", err)
			}
		}
		if !s.isUserOnline(ctx, recipientID) {
			s.publishOfflineNotification(ctx, recipientID, message.GetId(), message.GetConversationId(), message.GetSenderId(), "Ảnh", createdAt)
		}
	}
}

func (s *Server) GetMediaObject(ctx context.Context, req *pb.GetMediaObjectRequest) (*pb.GetMediaObjectResponse, error) {
	if s.db == nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	mediaID, userID := strings.TrimSpace(req.GetMediaId()), strings.TrimSpace(req.GetUserId())
	if _, err := uuid.Parse(mediaID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid media_id")
	}
	if _, err := uuid.Parse(userID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	result := &pb.GetMediaObjectResponse{}
	err := s.db.QueryRow(ctx, `
		SELECT mo.storage_key,mo.media_type,mo.byte_size
		FROM media_objects mo
		JOIN message_attachments ma ON ma.media_object_id=mo.id
		JOIN messages m ON m.id=ma.message_id AND m.deleted_at IS NULL
		JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 AND cm.left_at IS NULL
		WHERE mo.id=$1 AND mo.deleted_at IS NULL
		LIMIT 1`, mediaID, userID).Scan(&result.StorageKey, &result.MediaType, &result.ByteSize)
	if err != nil {
		return nil, dbError(err)
	}
	return result, nil
}

func (s *Server) isUserOnline(ctx context.Context, userID string) bool {
	if s.redis == nil {
		return false
	}
	exists, err := s.redis.Exists(ctx, "presence:user:"+userID).Result()
	return err == nil && exists > 0
}

func (s *Server) publishOfflineNotification(ctx context.Context, recipientID, messageID, conversationID, senderID, body string, createdAt time.Time) {
	if s.rabbit == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"type": "chat.message.offline", "recipient_id": recipientID,
		"message_id": messageID, "conversation_id": conversationID,
		"sender_id": senderID, "body": body,
		"created_at": createdAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return
	}
	_ = s.rabbit.PublishWithContext(ctx, "", "notification", false, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent,
		Timestamp: time.Now().UTC(), Body: payload,
	})
}

func (s *Server) GetOrCreateDirectConversation(ctx context.Context, req *pb.GetOrCreateDirectConversationRequest) (*pb.Conversation, error) {
	if s.db == nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	userID, recipientID := strings.TrimSpace(req.GetUserId()), strings.TrimSpace(req.GetRecipientId())
	if userID == "" || recipientID == "" || userID == recipientID {
		return nil, status.Error(codes.InvalidArgument, "two distinct user IDs are required")
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}
	parsedRecipientID, err := uuid.Parse(recipientID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid recipient ID")
	}
	userID, recipientID = parsedUserID.String(), parsedRecipientID.String()
	if userID == recipientID {
		return nil, status.Error(codes.InvalidArgument, "two distinct user IDs are required")
	}
	low, high := userID, recipientID
	if high < low {
		low, high = high, low
	}
	if s.friend == nil {
		return nil, status.Error(codes.Unavailable, "friend service unavailable")
	}
	friendCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	friendResult, friendErr := s.friend.IsFriend(friendCtx, &pb.IsFriendRequest{UserAId: userID, UserBId: recipientID})
	cancel()
	if friendErr != nil {
		return nil, status.Error(codes.Unavailable, "friend service unavailable")
	}
	if !friendResult.GetIsFriend() {
		return nil, status.Error(codes.PermissionDenied, "recipient is not a friend")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	defer tx.Rollback(ctx)
	var recipientExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND deleted_at IS NULL)`, recipientID).Scan(&recipientExists); err != nil {
		return nil, dbError(err)
	}
	if !recipientExists {
		return nil, status.Error(codes.NotFound, "recipient not found")
	}
	var conversationID string
	err = tx.QueryRow(ctx, `INSERT INTO conversations (type,created_by,direct_user_low,direct_user_high) VALUES ('direct',$1,$2,$3) ON CONFLICT (direct_user_low,direct_user_high) WHERE type='direct' DO NOTHING RETURNING id`, userID, low, high).Scan(&conversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM conversations WHERE type='direct' AND direct_user_low=$1 AND direct_user_high=$2`, low, high).Scan(&conversationID)
	}
	if err != nil {
		return nil, dbError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO conversation_members (conversation_id,user_id,role) VALUES ($1,$2,'owner'),($1,$3,'member') ON CONFLICT (conversation_id,user_id) DO NOTHING`, conversationID, userID, recipientID)
	if err != nil {
		return nil, dbError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, dbError(err)
	}
	return s.conversationForUser(ctx, conversationID, userID)
}

func (s *Server) ListConversations(ctx context.Context, req *pb.ListConversationsRequest) (*pb.ListConversationsResponse, error) {
	if s.db == nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.id,COALESCE(c.title,u.display_name,u.username,''),COALESCE(u.id::text,''),COALESCE(u.username,''),COALESCE(u.display_name,''),COALESCE(u.presence,'offline'),
		 COALESCE(lm.id::text,''),COALESCE(lm.body,''),COALESCE(lm.sender_id::text,''),COALESCE(lm.created_at,c.updated_at),
		 (SELECT COUNT(*) FROM message_deliveries d JOIN messages um ON um.id=d.message_id WHERE d.user_id=$1 AND d.delivered_at IS NULL AND um.conversation_id=c.id)
		FROM conversations c
		JOIN conversation_members mine ON mine.conversation_id=c.id AND mine.user_id=$1 AND mine.left_at IS NULL
		LEFT JOIN LATERAL (SELECT peer.user_id FROM conversation_members peer WHERE peer.conversation_id=c.id AND peer.user_id<>$1 AND peer.left_at IS NULL ORDER BY peer.joined_at LIMIT 1) p ON TRUE
		LEFT JOIN users u ON u.id=p.user_id
		LEFT JOIN LATERAL (SELECT m.id,m.body,m.sender_id,m.created_at FROM messages m WHERE m.conversation_id=c.id AND m.deleted_at IS NULL ORDER BY m.created_at DESC,m.id DESC LIMIT 1) lm ON TRUE
		ORDER BY COALESCE(lm.created_at,c.updated_at) DESC,c.id`, userID)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	conversations := make([]*pb.Conversation, 0)
	for rows.Next() {
		item := &pb.Conversation{}
		var updatedAt time.Time
		var unread int64
		if err := rows.Scan(&item.Id, &item.Title, &item.ParticipantId, &item.ParticipantUsername, &item.ParticipantDisplayName, &item.ParticipantPresence, &item.LastMessageId, &item.LastMessageBody, &item.LastMessageSenderId, &updatedAt, &unread); err != nil {
			return nil, dbError(err)
		}
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
		item.ParticipantPresence = s.participantPresence(ctx, item.ParticipantId, item.ParticipantPresence)
		item.UnreadCount = int32(unread)
		conversations = append(conversations, item)
	}
	if err := rows.Err(); err != nil {
		return nil, dbError(err)
	}
	return &pb.ListConversationsResponse{Conversations: conversations}, nil
}

func (s *Server) conversationForUser(ctx context.Context, conversationID, userID string) (*pb.Conversation, error) {
	item := &pb.Conversation{}
	var updatedAt time.Time
	var unread int64
	err := s.db.QueryRow(ctx, `SELECT c.id,COALESCE(c.title,u.display_name,u.username,''),COALESCE(u.id::text,''),COALESCE(u.username,''),COALESCE(u.display_name,''),COALESCE(u.presence,'offline'),COALESCE(lm.id::text,''),COALESCE(lm.body,''),COALESCE(lm.sender_id::text,''),COALESCE(lm.created_at,c.updated_at),(SELECT COUNT(*) FROM message_deliveries d JOIN messages um ON um.id=d.message_id WHERE d.user_id=$2 AND d.delivered_at IS NULL AND um.conversation_id=c.id) FROM conversations c JOIN conversation_members mine ON mine.conversation_id=c.id AND mine.user_id=$2 AND mine.left_at IS NULL LEFT JOIN LATERAL (SELECT peer.user_id FROM conversation_members peer WHERE peer.conversation_id=c.id AND peer.user_id<>$2 AND peer.left_at IS NULL ORDER BY peer.joined_at LIMIT 1) p ON TRUE LEFT JOIN users u ON u.id=p.user_id LEFT JOIN LATERAL (SELECT m.id,m.body,m.sender_id,m.created_at FROM messages m WHERE m.conversation_id=c.id AND m.deleted_at IS NULL ORDER BY m.created_at DESC,m.id DESC LIMIT 1) lm ON TRUE WHERE c.id=$1`, conversationID, userID).Scan(&item.Id, &item.Title, &item.ParticipantId, &item.ParticipantUsername, &item.ParticipantDisplayName, &item.ParticipantPresence, &item.LastMessageId, &item.LastMessageBody, &item.LastMessageSenderId, &updatedAt, &unread)
	if err != nil {
		return nil, dbError(err)
	}
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	item.ParticipantPresence = s.participantPresence(ctx, item.ParticipantId, item.ParticipantPresence)
	item.UnreadCount = int32(unread)
	return item, nil
}

func (s *Server) participantPresence(ctx context.Context, userID, fallback string) string {
	if userID == "" || s.redis == nil {
		return fallback
	}
	presence, err := s.redis.Get(ctx, "presence:user:"+userID).Result()
	if err != nil || presence == "" {
		return "offline"
	}
	return presence
}

func (s *Server) ListMessages(ctx context.Context, req *pb.ListMessagesRequest) (*pb.ListMessagesResponse, error) {
	if s.db == nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	conversationID, userID := strings.TrimSpace(req.GetConversationId()), strings.TrimSpace(req.GetUserId())
	if conversationID == "" || userID == "" {
		return nil, status.Error(codes.InvalidArgument, "conversation_id and user_id are required")
	}
	if _, err := uuid.Parse(conversationID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid conversation_id")
	}
	if _, err := uuid.Parse(userID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	limit := req.GetLimit()
	if limit < 1 || limit > 100 {
		limit = 50
	}
	var member bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL)`, conversationID, userID).Scan(&member); err != nil {
		return nil, dbError(err)
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "user is not an active conversation member")
	}
	rows, err := s.db.Query(ctx, `SELECT m.id,m.conversation_id,m.sender_id,COALESCE(m.body,''),m.created_at,mo.id,mo.media_type,mo.byte_size FROM (SELECT id,conversation_id,sender_id,body,created_at FROM messages WHERE conversation_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC,id DESC LIMIT $2) m LEFT JOIN message_attachments ma ON ma.message_id=m.id LEFT JOIN media_objects mo ON mo.id=ma.media_object_id AND mo.deleted_at IS NULL ORDER BY m.created_at DESC,m.id DESC,ma.position`, conversationID, limit)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	messages := make([]*pb.Message, 0, limit)
	messageByID := make(map[string]*pb.Message, limit)
	for rows.Next() {
		var itemID, itemConversationID, senderID, body string
		var createdAt time.Time
		var mediaID, mediaType *string
		var byteSize *int64
		if err := rows.Scan(&itemID, &itemConversationID, &senderID, &body, &createdAt, &mediaID, &mediaType, &byteSize); err != nil {
			return nil, dbError(err)
		}
		item := messageByID[itemID]
		if item == nil {
			item = &pb.Message{Id: itemID, ConversationId: itemConversationID, SenderId: senderID, Body: body, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano)}
			messageByID[itemID] = item
			messages = append(messages, item)
		}
		if mediaID != nil && mediaType != nil && byteSize != nil {
			item.Attachments = append(item.Attachments, &pb.MessageAttachment{Id: *mediaID, MediaType: *mediaType, ByteSize: *byteSize, Url: "/api/v1/media/" + *mediaID})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, dbError(err)
	}
	return &pb.ListMessagesResponse{Messages: messages}, nil
}

func (s *Server) ListUnreadMessages(ctx context.Context, req *pb.ListUnreadMessagesRequest) (*pb.ListUnreadMessagesResponse, error) {
	if s.db == nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	limit := req.GetLimit()
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `SELECT m.id,m.conversation_id,m.sender_id,COALESCE(m.body,''),m.created_at FROM message_deliveries d JOIN messages m ON m.id=d.message_id JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=d.user_id AND cm.left_at IS NULL WHERE d.user_id=$1 AND d.delivered_at IS NULL AND m.deleted_at IS NULL ORDER BY m.created_at DESC,m.id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	messages := make([]*pb.UnreadMessage, 0)
	for rows.Next() {
		var item pb.UnreadMessage
		var createdAt time.Time
		if err := rows.Scan(&item.MessageId, &item.ConversationId, &item.SenderId, &item.Body, &createdAt); err != nil {
			return nil, dbError(err)
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		messages = append(messages, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, dbError(err)
	}
	return &pb.ListUnreadMessagesResponse{Messages: messages}, nil
}

func (s *Server) MarkDelivered(ctx context.Context, req *pb.MarkDeliveredRequest) (*pb.MarkDeliveredResponse, error) {
	if s.db == nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	userID, messageID := strings.TrimSpace(req.GetUserId()), strings.TrimSpace(req.GetMessageId())
	if userID == "" || messageID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and message_id are required")
	}
	if _, err := uuid.Parse(userID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	if _, err := uuid.Parse(messageID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid message_id")
	}
	result, err := s.db.Exec(ctx, `UPDATE message_deliveries SET delivered_at=COALESCE(delivered_at,NOW()) WHERE message_id=$1 AND user_id=$2`, messageID, userID)
	if err != nil {
		return nil, dbError(err)
	}
	if result.RowsAffected() == 0 {
		var member bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id WHERE m.id=$1 AND cm.user_id=$2 AND cm.left_at IS NULL)`, messageID, userID).Scan(&member); err != nil {
			return nil, dbError(err)
		}
		if !member {
			return nil, status.Error(codes.NotFound, "message delivery not found")
		}
	}
	return &pb.MarkDeliveredResponse{Success: true}, nil
}

func dbError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, "chat record not found")
	}
	return status.Error(codes.Internal, "chat database operation failed")
}
