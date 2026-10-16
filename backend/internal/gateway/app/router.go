package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/thehien0811/chatter/config"
	"github.com/thehien0811/chatter/internal/auth/app"
	"github.com/thehien0811/chatter/internal/auth/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// App is the API gateway bootstrap for the hexagonal structure.
type App struct {
	engine                     *gin.Engine
	jwt                        *app.JWTService
	casbin                     *app.CasbinService
	authUC                     *usecase.AuthUseCase
	services                   *ServiceClients
	redisClient                *redis.Client
	secret                     string
	allowLocalWebSocketOrigins bool
	hub                        *websocketHub
	objects                    *objectStorage
	objectStorageErr           error
}

func NewApp(secret string) *App {
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.AppConfig{}
		cfg.JWT.Secret = "phase-1-default-secret"
		cfg.App.Port = "8080"
	}
	if secret == "" {
		secret = cfg.JWT.Secret
	}
	if secret == "" {
		secret = "phase-1-default-secret"
	}

	jwtSvc := app.NewJWTService(secret)
	casbinSvc := app.NewCasbinService()
	authUC := usecase.NewAuthUseCase(jwtSvc, casbinSvc)
	redisClient := app.NewRedisClient(cfg)
	services, _ := NewServiceClients(cfg)
	objects, objectStorageErr := newObjectStorage(
		cfg.MinIO.Endpoint, cfg.MinIO.AccessKey, cfg.MinIO.SecretKey,
		cfg.MinIO.Bucket, cfg.MinIO.Secure,
	)
	if objectStorageErr != nil {
		log.Printf("image storage unavailable: %v", objectStorageErr)
	}

	gateway := &App{
		engine:                     gin.Default(),
		jwt:                        jwtSvc,
		casbin:                     casbinSvc,
		authUC:                     authUC,
		services:                   services,
		redisClient:                redisClient,
		secret:                     secret,
		allowLocalWebSocketOrigins: strings.EqualFold(cfg.App.Env, "development"),
		hub:                        newWebsocketHub(redisClient),
		objects:                    objects,
		objectStorageErr:           objectStorageErr,
	}
	gateway.registerRoutes()
	return gateway
}

func (a *App) Engine() *gin.Engine {
	return a.engine
}

func (a *App) registerRoutes() {
	api := a.engine.Group("/api/v1")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api.POST("/auth/register", a.registerHandler)
	api.POST("/auth/login", a.loginHandler)
	api.POST("/auth/refresh", a.refreshHandler)
	api.GET("/users/:id/status", a.userStatusHandler)
	api.GET("/ws/presence", a.websocketPresenceHandler)

	protected := api.Group("")
	protected.Use(a.requireAuth(), a.authorize())
	protected.GET("/profile", a.profileHandler)
	protected.GET("/friends", a.listFriendsHandler)
	protected.GET("/friend-requests", a.listFriendRequestsHandler)
	protected.POST("/friend-requests", a.sendFriendRequestHandler)
	protected.POST("/friend-requests/:id/accept", a.respondFriendRequestHandler(true))
	protected.POST("/friend-requests/:id/reject", a.respondFriendRequestHandler(false))
	protected.GET("/users/search", a.searchUsersHandler)
	protected.GET("/chat/unread", a.unreadMessagesHandler)
	protected.GET("/chat/conversations", a.listConversationsHandler)
	protected.POST("/chat/conversations", a.createConversationHandler)
	protected.GET("/chat/conversations/:id/messages", a.listMessagesHandler)
	protected.POST("/chat/images", a.uploadImageMessageHandler)
	protected.GET("/media/:id", a.getMediaHandler)
	protected.POST("/chat/messages/:id/delivered", a.markMessageDeliveredHandler)
	protected.PATCH("/profile", a.updateProfileHandler)
	protected.GET("/users/:id", a.userProfileHandler)
	protected.PATCH("/users/:id", a.userUpdateHandler)
	protected.GET("/admin/users", a.adminUsersHandler)
}

func (a *App) registerHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "user/auth services unavailable"})
		return
	}
	var payload struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Username == "" || payload.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}
	user, err := a.services.Register(c.Request.Context(), payload.Username, payload.DisplayName, payload.Email, payload.Password)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "user service registration failed"})
		return
	}
	tokens, err := a.services.Login(c.Request.Context(), payload.Username, payload.Password)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "account created but auth service login failed"})
		return
	}
	c.JSON(http.StatusCreated, authResponse(user, tokens))
}

func (a *App) loginHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth/user services unavailable"})
		return
	}
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Username == "" || payload.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}
	tokens, err := a.services.Login(c.Request.Context(), payload.Username, payload.Password)
	if err != nil {
		code := status.Code(err)
		if code == codes.Unauthenticated || code == codes.InvalidArgument {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "auth service unavailable"})
		return
	}
	user, err := a.services.GetUser(c.Request.Context(), tokens.UserID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "user service unavailable"})
		return
	}
	c.JSON(http.StatusOK, authResponse(user, tokens))
}

func userResponse(user *RemoteUser) gin.H {
	return gin.H{"id": user.ID, "username": user.Username, "display_name": user.DisplayName,
		"email": user.Email, "presence": user.Presence, "status_message": user.StatusMessage}
}

func authResponse(user *RemoteUser, tokens *AuthTokens) gin.H {
	return gin.H{"user": userResponse(user), "access_token": tokens.AccessToken,
		"refresh_token": tokens.RefreshToken, "token_type": "Bearer", "expires_in": 7200}
}

func (a *App) handleSocketMessage(ctx context.Context, userID string, conn *websocket.Conn, payload []byte) error {
	if a.services == nil {
		return errors.New("chat service unavailable")
	}
	var request struct {
		Type           string          `json:"type"`
		ConversationID string          `json:"conversation_id"`
		Body           string          `json:"body"`
		Payload        json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		return err
	}
	if strings.HasPrefix(request.Type, "call.") {
		if err := a.handleCallSignal(ctx, userID, payload); err != nil {
			log.Printf("reject call signaling event %s: %v", request.Type, err)
			var callPayload struct {
				CallID string `json:"callId"`
			}
			if err := json.Unmarshal(request.Payload, &callPayload); err != nil {
				log.Printf("read rejected call signaling payload: %v", err)
			}
			response, _ := json.Marshal(gin.H{
				"type": "call.error",
				"payload": gin.H{
					"callId": callPayload.CallID,
					"error":  "call is unavailable or permission was denied",
				},
				"ts": time.Now().UnixMilli(),
			})
			return a.hub.writeConnection(conn, response)
		}
		return nil
	}
	if request.Type != "send_message" || request.ConversationID == "" || request.Body == "" {
		return errors.New("invalid message")
	}
	messageID, err := a.services.SendMessage(ctx, request.ConversationID, userID, request.Body)
	if err != nil {
		return err
	}
	ack, _ := json.Marshal(gin.H{"type": "message.accepted", "message_id": messageID})
	return a.hub.writeConnection(conn, ack)
}

func (a *App) searchUsersHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "friend service unavailable"})
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	if len([]rune(query)) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q must contain at least 2 characters"})
		return
	}
	userID, _ := c.Get("user_id")
	users, err := a.services.SearchUsers(c.Request.Context(), userID.(string), query)
	if err != nil {
		c.JSON(friendErrorStatus(err), gin.H{"error": "user search failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (a *App) listFriendsHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "friend service unavailable"})
		return
	}
	userID, _ := c.Get("user_id")
	friends, err := a.services.ListFriends(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "friend service unavailable"})
		return
	}
	result := make([]gin.H, 0, len(friends))
	for _, friend := range friends {
		presence, presenceErr := a.getUserPresence(c.Request.Context(), friend.ID)
		if presenceErr != nil {
			log.Printf("read presence while listing friends: %v", presenceErr)
			presence.Status = "unknown"
		}
		result = append(result, gin.H{
			"id": friend.ID, "username": friend.Username, "display_name": friend.DisplayName,
			"presence": presence.Status, "presence_version": presence.Version,
		})
	}
	c.JSON(http.StatusOK, gin.H{"friends": result})
}

func (a *App) listFriendRequestsHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "friend service unavailable"})
		return
	}
	direction := strings.ToLower(strings.TrimSpace(c.DefaultQuery("direction", "incoming")))
	if direction != "incoming" && direction != "outgoing" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "direction must be incoming or outgoing"})
		return
	}
	userID, _ := c.Get("user_id")
	requests, err := a.services.ListFriendRequests(c.Request.Context(), userID.(string), direction)
	if err != nil {
		c.JSON(friendErrorStatus(err), gin.H{"error": "friend request list failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

func (a *App) sendFriendRequestHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "friend service unavailable"})
		return
	}
	var payload struct {
		RecipientID string `json:"recipient_id"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || strings.TrimSpace(payload.RecipientID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recipient_id is required"})
		return
	}
	userID, _ := c.Get("user_id")
	request, err := a.services.SendFriendRequest(c.Request.Context(), userID.(string), payload.RecipientID)
	if err != nil {
		c.JSON(friendErrorStatus(err), gin.H{"error": "could not send friend request"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"request": request})
}

func (a *App) respondFriendRequestHandler(accept bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.services == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "friend service unavailable"})
			return
		}
		userID, _ := c.Get("user_id")
		request, err := a.services.RespondFriendRequest(c.Request.Context(), c.Param("id"), userID.(string), accept)
		if err != nil {
			c.JSON(friendErrorStatus(err), gin.H{"error": "could not update friend request"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"request": request})
	}
}

func friendErrorStatus(err error) int {
	switch status.Code(err) {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.FailedPrecondition:
		return http.StatusConflict
	default:
		return http.StatusBadGateway
	}
}

func (a *App) listConversationsHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat service unavailable"})
		return
	}
	userID, _ := c.Get("user_id")
	conversations, err := a.services.ListConversations(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "chat service unavailable"})
		return
	}
	a.enrichConversationPresence(c.Request.Context(), conversations)
	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

func (a *App) enrichConversationPresence(ctx context.Context, conversations []ChatConversation) {
	for index := range conversations {
		a.enrichOneConversationPresence(ctx, &conversations[index])
	}
}

func (a *App) enrichOneConversationPresence(ctx context.Context, conversation *ChatConversation) {
	presence, err := a.getUserPresence(ctx, conversation.Participant.ID)
	if err != nil {
		log.Printf("read conversation participant presence: %v", err)
		conversation.Participant.Presence = "unknown"
		return
	}
	conversation.Participant.Presence = presence.Status
	conversation.Participant.PresenceVersion = presence.Version
}

func (a *App) createConversationHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat service unavailable"})
		return
	}
	var payload struct {
		RecipientID string `json:"recipient_id"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || strings.TrimSpace(payload.RecipientID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recipient_id is required"})
		return
	}
	userID, _ := c.Get("user_id")
	conversation, err := a.services.GetOrCreateDirectConversation(c.Request.Context(), userID.(string), payload.RecipientID)
	if err != nil {
		if status.Code(err) == codes.InvalidArgument {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recipient_id"})
			return
		}
		if status.Code(err) == codes.PermissionDenied {
			c.JSON(http.StatusForbidden, gin.H{"error": "recipient must be your friend"})
			return
		}
		if status.Code(err) == codes.NotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "recipient not found"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "chat service unavailable"})
		return
	}
	a.enrichOneConversationPresence(c.Request.Context(), conversation)
	c.JSON(http.StatusOK, conversation)
}

func (a *App) listMessagesHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat service unavailable"})
		return
	}
	limit := int64(50)
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil || parsed < 1 || parsed > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 100"})
			return
		}
		limit = parsed
	}
	userID, _ := c.Get("user_id")
	currentUserID := userID.(string)
	messages, err := a.services.ListMessages(c.Request.Context(), c.Param("id"), currentUserID, int32(limit))
	if err != nil {
		if status.Code(err) == codes.InvalidArgument {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation ID"})
			return
		}
		if status.Code(err) == codes.PermissionDenied {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a conversation member"})
			return
		}
		if status.Code(err) == codes.NotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "chat service unavailable"})
		return
	}
	for _, message := range messages {
		if message.SenderID != currentUserID {
			if err := a.services.MarkDelivered(c.Request.Context(), currentUserID, message.ID); err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": "failed to mark messages delivered"})
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func (a *App) markMessageDeliveredHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat service unavailable"})
		return
	}
	userID, _ := c.Get("user_id")
	if err := a.services.MarkDelivered(c.Request.Context(), userID.(string), c.Param("id")); err != nil {
		if status.Code(err) == codes.InvalidArgument {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message ID"})
			return
		}
		if status.Code(err) == codes.NotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "message delivery not found"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "chat service unavailable"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) unreadMessagesHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat service unavailable"})
		return
	}
	limit := int64(50)
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil || parsed < 1 || parsed > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 100"})
			return
		}
		limit = parsed
	}
	userID, _ := c.Get("user_id")
	messages, err := a.services.ListUnread(c.Request.Context(), userID.(string), int32(limit))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "chat service unavailable"})
		return
	}
	for _, message := range messages {
		if err := a.services.MarkDelivered(c.Request.Context(), userID.(string), message.MessageID); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to mark messages delivered"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func (a *App) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}

		claims, err := a.jwt.Validate(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_role", claims.Role)
		c.Next()
	}
}

func (a *App) authorize() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := c.Get("user_role")
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing role"})
			return
		}

		allowed, err := a.authUC.Authorize(role.(string), c.Request.URL.Path, c.Request.Method)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if c.Request.Method == http.MethodPatch && c.Param("id") != "" && role.(string) != "admin" {
			userID, _ := c.Get("user_id")
			if c.Param("id") != userID.(string) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cannot update another user"})
				return
			}
		}

		c.Next()
	}
}

func (a *App) refreshHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth service unavailable"})
		return
	}
	var payload struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || payload.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}
	tokens, err := a.services.Refresh(c.Request.Context(), payload.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": tokens.AccessToken, "refresh_token": tokens.RefreshToken, "token_type": "Bearer", "expires_in": 7200})
}

func (a *App) profileHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "user service unavailable"})
		return
	}
	userID, _ := c.Get("user_id")
	user, err := a.services.GetUser(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, userResponse(user))
}

func (a *App) updateProfileHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "user service unavailable"})
		return
	}
	userID, _ := c.Get("user_id")
	var payload struct {
		DisplayName   string `json:"display_name"`
		Email         string `json:"email"`
		StatusMessage string `json:"status_message"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	user, err := a.services.UpdateUser(c.Request.Context(), userID.(string), payload.DisplayName, payload.Email, payload.StatusMessage)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "user service update failed"})
		return
	}
	c.JSON(http.StatusOK, userResponse(user))
}

func (a *App) userStatusHandler(c *gin.Context) {
	userID := c.Param("id")
	presence, err := a.getUserPresence(c.Request.Context(), userID)
	if err != nil {
		log.Printf("read user presence: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "presence service unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id": userID, "status": presence.Status, "version": presence.Version,
	})
}

func (a *App) websocketPresenceHandler(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		token = strings.TrimSpace(c.GetHeader("Authorization"))
		if len(token) >= 7 && strings.EqualFold(token[:7], "Bearer ") {
			token = strings.TrimSpace(token[7:])
		}
	}
	claims, err := a.jwt.Validate(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid token"})
		return
	}
	allowed, err := a.authUC.Authorize(claims.Role, c.Request.URL.Path, c.Request.Method)
	if err != nil || !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" || strings.EqualFold(origin, "https://"+r.Host) || strings.EqualFold(origin, "http://"+r.Host) {
				return true
			}
			if a.allowLocalWebSocketOrigins {
				return strings.EqualFold(origin, "http://localhost:3000") || strings.EqualFold(origin, "http://127.0.0.1:3000")
			}
			return false
		},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	userID := claims.UserID
	unregister, err := a.hub.register(userID, conn)
	if err != nil {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "message service unavailable"))
		return
	}
	connectionID := uuid.NewString()
	version, err := a.updatePresenceConnection(c.Request.Context(), userID, connectionID, true)
	if err != nil {
		unregister()
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "presence update failed"))
		return
	}
	if err := a.publishPresenceVersion(c.Request.Context(), userID, "online", version); err != nil {
		log.Printf("publish user online presence: %v", err)
	}
	defer func() {
		unregister()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		version, err := a.updatePresenceConnection(ctx, userID, connectionID, false)
		if err != nil {
			log.Printf("remove websocket presence lease: %v", err)
			return
		}
		if err := a.publishPresenceVersion(ctx, userID, "offline", version); err != nil {
			log.Printf("publish user offline presence: %v", err)
		}
	}()

	conn.SetReadLimit(64 * 1024)
	if err := conn.SetReadDeadline(time.Now().Add(presenceLeaseTTL)); err != nil {
		return
	}
	conn.SetPongHandler(func(string) error {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		version, err := a.updatePresenceConnection(ctx, userID, connectionID, true)
		if err != nil {
			return err
		}
		if err := a.publishPresenceVersion(ctx, userID, "online", version); err != nil {
			log.Printf("publish refreshed online presence: %v", err)
		}
		return conn.SetReadDeadline(time.Now().Add(presenceLeaseTTL))
	})

	done := make(chan struct{})
	defer close(done)
	ticker := time.NewTicker(websocketPingInterval)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := a.hub.writeControl(userID, conn, websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					_ = conn.Close()
					return
				}
			case <-done:
				return
			}
		}
	}()
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		version, err := a.updatePresenceConnection(ctx, userID, connectionID, true)
		cancel()
		if err != nil {
			log.Printf("refresh websocket presence lease: %v", err)
			return
		}
		if err := a.publishPresenceVersion(c.Request.Context(), userID, "online", version); err != nil {
			log.Printf("publish refreshed online presence: %v", err)
		}
		if err := conn.SetReadDeadline(time.Now().Add(presenceLeaseTTL)); err != nil {
			return
		}
		if err := a.handleSocketMessage(c.Request.Context(), userID, conn, payload); err != nil {
			message, _ := json.Marshal(gin.H{"type": "error", "error": "message could not be sent"})
			if err := a.hub.writeConnection(conn, message); err != nil {
				return
			}
		}
	}
}

func (a *App) userProfileHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "user service unavailable"})
		return
	}
	user, err := a.services.GetUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, userResponse(user))
}

func (a *App) userUpdateHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "user service unavailable"})
		return
	}
	var payload struct {
		DisplayName   string `json:"display_name"`
		Email         string `json:"email"`
		StatusMessage string `json:"status_message"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	user, err := a.services.UpdateUser(c.Request.Context(), c.Param("id"), payload.DisplayName, payload.Email, payload.StatusMessage)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "user service update failed"})
		return
	}
	c.JSON(http.StatusOK, userResponse(user))
}

func (a *App) adminUsersHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"users": []string{"alice", "bob", "charlie"}})
}

func (a *App) Run(addr string) error {
	if a.services != nil {
		defer a.services.Close()
	}
	return a.engine.Run(addr)
}

var ErrUnauthorized = errors.New("unauthorized")
