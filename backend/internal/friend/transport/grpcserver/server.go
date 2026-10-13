package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
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

const friendCacheTTL = 5 * time.Minute

// Server owns friend request state and the canonical one-row-per-pair friendship relation.
type Server struct {
	pb.UnimplementedFriendServiceServer
	db    *pgxpool.Pool
	redis *redis.Client
	amqp  *amqp.Channel
}

func New(db *pgxpool.Pool, redisClient *redis.Client, rabbit *amqp.Channel) *Server {
	return &Server{db: db, redis: redisClient, amqp: rabbit}
}

type friendEvent struct {
	Type    string            `json:"type"`
	Request *pb.FriendRequest `json:"request,omitempty"`
}

func (s *Server) SendFriendRequest(ctx context.Context, req *pb.SendFriendRequestRequest) (*pb.FriendRequest, error) {
	fromID, toID, err := validPair(req.GetFromUserId(), req.GetToUserId())
	if err != nil {
		return nil, err
	}
	var usersExist bool
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*)=2 FROM users WHERE id IN ($1,$2) AND deleted_at IS NULL`, fromID, toID).Scan(&usersExist); err != nil {
		return nil, dbError(err)
	}
	if !usersExist {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	var reversePending bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM friend_requests WHERE sender_id=$2 AND recipient_id=$1 AND status='pending')`, fromID, toID).Scan(&reversePending); err != nil {
		return nil, dbError(err)
	}
	if reversePending {
		return nil, status.Error(codes.AlreadyExists, "the other user already sent you a friend request")
	}
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM friendships WHERE user_a_id=LEAST($1::uuid,$2::uuid) AND user_b_id=GREATEST($1::uuid,$2::uuid))`, fromID, toID).Scan(&exists); err != nil {
		return nil, dbError(err)
	}
	if exists {
		return nil, status.Error(codes.AlreadyExists, "users are already friends")
	}
	var item pb.FriendRequest
	var created time.Time
	err = s.db.QueryRow(ctx, `INSERT INTO friend_requests(sender_id,recipient_id,status) VALUES($1,$2,'pending') RETURNING id,created_at`, fromID, toID).Scan(&item.Id, &created)
	if err != nil {
		if isUnique(err) {
			return nil, status.Error(codes.AlreadyExists, "friend request is already pending")
		}
		return nil, dbError(err)
	}
	item.Status, item.CreatedAt = "pending", created.UTC().Format(time.RFC3339Nano)
	item.FromUser, item.ToUser, err = s.loadRequestUsers(ctx, fromID, toID)
	if err != nil {
		return nil, err
	}
	s.publish(ctx, "friend.request", &item, toID)
	return &item, nil
}

func (s *Server) ListFriendRequests(ctx context.Context, req *pb.ListFriendRequestsRequest) (*pb.ListFriendRequestsResponse, error) {
	userID, err := validID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	direction := strings.TrimSpace(strings.ToLower(req.GetDirection()))
	if direction == "" {
		direction = "incoming"
	}
	var filter string
	switch direction {
	case "incoming":
		filter = "r.recipient_id=$1"
	case "outgoing":
		filter = "r.sender_id=$1"
	default:
		return nil, status.Error(codes.InvalidArgument, "direction must be incoming or outgoing")
	}
	rows, err := s.db.Query(ctx, `SELECT r.id,r.sender_id,r.recipient_id,r.status,r.created_at FROM friend_requests r WHERE `+filter+` AND r.status='pending' ORDER BY r.created_at DESC,r.id`, userID)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	out := &pb.ListFriendRequestsResponse{Requests: make([]*pb.FriendRequest, 0)}
	for rows.Next() {
		item := &pb.FriendRequest{}
		var fromID, toID string
		var created time.Time
		if err := rows.Scan(&item.Id, &fromID, &toID, &item.Status, &created); err != nil {
			return nil, dbError(err)
		}
		item.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		item.FromUser, item.ToUser, err = s.loadRequestUsers(ctx, fromID, toID)
		if err != nil {
			return nil, err
		}
		out.Requests = append(out.Requests, item)
	}
	if err := rows.Err(); err != nil {
		return nil, dbError(err)
	}
	return out, nil
}

func (s *Server) RespondFriendRequest(ctx context.Context, req *pb.RespondFriendRequestRequest) (*pb.FriendRequest, error) {
	requestID, err := validID(req.GetRequestId())
	if err != nil {
		return nil, err
	}
	userID, err := validID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "database unavailable")
	}
	defer tx.Rollback(ctx)
	var fromID, toID string
	var item pb.FriendRequest
	var created time.Time
	err = tx.QueryRow(ctx, `SELECT id,sender_id,recipient_id,status,created_at FROM friend_requests WHERE id=$1 AND recipient_id=$2 FOR UPDATE`, requestID, userID).Scan(&item.Id, &fromID, &toID, &item.Status, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "pending friend request not found")
	}
	if err != nil {
		return nil, dbError(err)
	}
	if item.Status != "pending" {
		return nil, status.Error(codes.FailedPrecondition, "friend request is no longer pending")
	}
	if req.GetAccept() {
		if _, err = tx.Exec(ctx, `UPDATE friend_requests SET status='accepted',responded_at=NOW() WHERE id=$1`, requestID); err != nil {
			return nil, dbError(err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO friendships(user_a_id,user_b_id,created_from_request_id) VALUES(LEAST($1::uuid,$2::uuid),GREATEST($1::uuid,$2::uuid),$3) ON CONFLICT(user_a_id,user_b_id) DO NOTHING`, fromID, toID, requestID); err != nil {
			return nil, dbError(err)
		}
		item.Status = "accepted"
	} else {
		if _, err = tx.Exec(ctx, `UPDATE friend_requests SET status='rejected',responded_at=NOW() WHERE id=$1`, requestID); err != nil {
			return nil, dbError(err)
		}
		item.Status = "rejected"
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, dbError(err)
	}
	item.CreatedAt = created.UTC().Format(time.RFC3339Nano)
	item.FromUser, item.ToUser, err = s.loadRequestUsers(ctx, fromID, toID)
	if err != nil {
		return nil, err
	}
	if item.Status == "accepted" {
		s.invalidateFriendCache(ctx, fromID, toID)
	}
	s.publish(ctx, "friend."+item.Status, &item, fromID, toID)
	return &item, nil
}

func (s *Server) ListFriends(ctx context.Context, req *pb.ListFriendsRequest) (*pb.ListFriendsResponse, error) {
	userID, err := validID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT u.id,u.username,u.display_name FROM friendships f JOIN users u ON u.id=CASE WHEN f.user_a_id=$1 THEN f.user_b_id ELSE f.user_a_id END WHERE (f.user_a_id=$1 OR f.user_b_id=$1) AND u.deleted_at IS NULL ORDER BY u.username,u.id`, userID)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	out := &pb.ListFriendsResponse{Friends: make([]*pb.FriendUser, 0)}
	for rows.Next() {
		u := &pb.FriendUser{}
		if err := rows.Scan(&u.Id, &u.Username, &u.DisplayName); err != nil {
			return nil, dbError(err)
		}
		out.Friends = append(out.Friends, u)
	}
	if err := rows.Err(); err != nil {
		return nil, dbError(err)
	}
	return out, nil
}

func (s *Server) IsFriend(ctx context.Context, req *pb.IsFriendRequest) (*pb.IsFriendResponse, error) {
	a, b, err := validPair(req.GetUserAId(), req.GetUserBId())
	if err != nil {
		return nil, err
	}
	key := "friends:" + a
	if s.redis != nil {
		cached, e := s.redis.SIsMember(ctx, key, b).Result()
		if e == nil && cached {
			return &pb.IsFriendResponse{IsFriend: true}, nil
		}
		// Trust positive cache entries only. Friendships may be written by
		// administrative SQL or another integration that cannot invalidate this
		// service's Redis key, so a cached negative could incorrectly deny chat.
	}
	var yes bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM friendships WHERE user_a_id=LEAST($1::uuid,$2::uuid) AND user_b_id=GREATEST($1::uuid,$2::uuid))`, a, b).Scan(&yes); err != nil {
		return nil, dbError(err)
	}
	if s.redis != nil {
		_ = s.cacheFriends(ctx, a)
		_ = s.cacheFriends(ctx, b)
	}
	return &pb.IsFriendResponse{IsFriend: yes}, nil
}

func (s *Server) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest) (*pb.SearchUsersResponse, error) {
	query := strings.TrimSpace(req.GetQuery())
	if len([]rune(query)) < 2 {
		return nil, status.Error(codes.InvalidArgument, "query must contain at least 2 characters")
	}
	exclude, err := validID(req.GetExcludeUserId())
	if err != nil {
		return nil, err
	}
	limit := req.GetLimit()
	if limit < 1 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.Query(ctx, `SELECT id,username,display_name FROM users WHERE deleted_at IS NULL AND id<>$1 AND (username ILIKE $2 OR display_name ILIKE $2) ORDER BY username,id LIMIT $3`, exclude, "%"+escapeLike(query)+"%", limit)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	out := &pb.SearchUsersResponse{Users: make([]*pb.FriendUser, 0)}
	for rows.Next() {
		u := &pb.FriendUser{}
		if err := rows.Scan(&u.Id, &u.Username, &u.DisplayName); err != nil {
			return nil, dbError(err)
		}
		out.Users = append(out.Users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, dbError(err)
	}
	return out, nil
}

func (s *Server) loadRequestUsers(ctx context.Context, fromID, toID string) (*pb.FriendUser, *pb.FriendUser, error) {
	users := make([]*pb.FriendUser, 0, 2)
	rows, err := s.db.Query(ctx, `SELECT id,username,display_name FROM users WHERE id=$1 OR id=$2`, fromID, toID)
	if err != nil {
		return nil, nil, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		u := &pb.FriendUser{}
		if err := rows.Scan(&u.Id, &u.Username, &u.DisplayName); err != nil {
			return nil, nil, dbError(err)
		}
		users = append(users, u)
	}
	var from, to *pb.FriendUser
	for _, u := range users {
		if u.Id == fromID {
			from = u
		}
		if u.Id == toID {
			to = u
		}
	}
	if from == nil || to == nil {
		return nil, nil, status.Error(codes.NotFound, "friend request user not found")
	}
	return from, to, nil
}

func (s *Server) publish(ctx context.Context, eventType string, item *pb.FriendRequest, userIDs ...string) {
	payload, err := json.Marshal(friendEvent{Type: eventType, Request: item})
	if err != nil {
		return
	}
	if s.redis != nil {
		for _, id := range userIDs {
			_ = s.redis.Publish(ctx, "friend:user:"+id, payload).Err()
		}
	}
	if s.amqp != nil {
		_ = s.amqp.PublishWithContext(ctx, "friend.events", "friend.notification", false, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Timestamp: time.Now().UTC(), Body: payload})
	}
}
func (s *Server) cacheFriends(ctx context.Context, userID string) error {
	if s.redis == nil {
		return nil
	}
	key := "friends:" + userID
	pipe := s.redis.TxPipeline()
	pipe.Del(ctx, key)
	pipe.SAdd(ctx, key, "__friends_loaded__")
	rows, err := s.db.Query(ctx, `SELECT CASE WHEN user_a_id=$1 THEN user_b_id ELSE user_a_id END FROM friendships WHERE user_a_id=$1 OR user_b_id=$1`, userID)
	if err != nil {
		return err
	}
	members := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		members = append(members, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(members) > 0 {
		args := make([]interface{}, 0, len(members)+1)
		args = append(args, "__friends_loaded__")
		for _, id := range members {
			args = append(args, id)
		}
		pipe.SAdd(ctx, key, args...)
	}
	pipe.Expire(ctx, key, friendCacheTTL)
	_, err = pipe.Exec(ctx)
	return err
}
func (s *Server) invalidateFriendCache(ctx context.Context, ids ...string) {
	if s.redis == nil {
		return
	}
	for _, id := range ids {
		_ = s.redis.Del(ctx, "friends:"+id).Err()
	}
}
func (s *Server) Close() {
	if s.amqp != nil {
		_ = s.amqp.Close()
	}
}
func validID(raw string) (string, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", status.Error(codes.InvalidArgument, "invalid user or request ID")
	}
	return id.String(), nil
}
func validPair(a, b string) (string, string, error) {
	aa, err := validID(a)
	if err != nil {
		return "", "", err
	}
	bb, err := validID(b)
	if err != nil {
		return "", "", err
	}
	if aa == bb {
		return "", "", status.Error(codes.InvalidArgument, "two distinct user IDs are required")
	}
	return aa, bb, nil
}
func escapeLike(v string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(v)
}
func isUnique(err error) bool {
	return err != nil && (strings.Contains(strings.ToLower(err.Error()), "unique constraint") || strings.Contains(strings.ToLower(err.Error()), "duplicate key"))
}
func dbError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, "friend record not found")
	}
	return status.Error(codes.Internal, "friend database operation failed")
}
