# Chatter API (hiện trạng)

Tài liệu này mô tả API mà API Gateway hiện khai báo trong Gin router và RPC contract nội bộ hiện có trong `proto/`. Đây là trạng thái code hiện tại; một số service nghiệp vụ mới là scaffold.

## Kết nối

- Base URL local: `http://localhost:8080`
- REST prefix: `/api/v1`
- JSON encoding: `application/json`
- WebSocket URL: `ws://localhost:8080/api/v1/ws/presence?token=<access_token>`
- Chỉ API Gateway publish port ra host theo `docker-compose.yml`. Các RPC gRPC là nội bộ Docker network.

## REST endpoints

| Method | Path | Quyền | Trạng thái |
| --- | --- | --- | --- |
| `GET` | `/api/v1/health` | Công khai | Hoạt động, trả trạng thái gateway |
| `POST` | `/api/v1/auth/register` | Công khai | Tạo user qua User Service rồi đăng nhập qua Auth Service |
| `POST` | `/api/v1/auth/login` | Công khai | Đăng nhập qua Auth Service, tải profile qua User Service |
| `POST` | `/api/v1/auth/refresh` | Công khai, yêu cầu refresh token | Đổi refresh token lấy cặp token mới |
| `GET` | `/api/v1/users/:id/status` | Công khai | Đọc presence từ Redis |
| `GET` | `/api/v1/profile` | JWT Bearer + Casbin | Hoạt động |
| `GET` | `/api/v1/users/:id` | JWT Bearer + Casbin | Hoạt động |
| `PATCH` | `/api/v1/users/:id` | JWT Bearer + Casbin | Chỉ admin hoặc user sửa chính mình |
| `GET` | `/api/v1/chat/conversations` | JWT Bearer + Casbin | Danh sách hội thoại của user |
| `POST` | `/api/v1/chat/conversations` | JWT Bearer + Casbin | Tạo hoặc lấy hội thoại trực tiếp với một người bạn |
| `GET` | `/api/v1/chat/conversations/:id/messages` | JWT Bearer + Casbin | Lịch sử tin nhắn, chỉ thành viên hội thoại |
| `POST` | `/api/v1/chat/messages/:id/delivered` | JWT Bearer + Casbin | Đánh dấu tin nhắn đã giao cho user hiện tại |
| `GET` | `/api/v1/chat/unread` | JWT Bearer + Casbin | Đọc tin nhắn đang chờ giao và đánh dấu delivered |
| `GET` | `/api/v1/ws/presence` | JWT + Casbin; nâng cấp thành WebSocket | Presence và gửi/nhận realtime qua Redis Pub/Sub |
| `POST` | `/api/v1/chat/images` | JWT + Casbin | Upload ảnh tin nhắn, giới hạn 10 MB |
| `GET` | `/api/v1/media/:id` | JWT + Casbin | Đọc ảnh nếu là thành viên hội thoại chứa ảnh |
| `PATCH` | `/api/v1/profile` | JWT Bearer | Route có handler nhưng policy Casbin chưa cho phép, hiện trả `403` |
| `GET` | `/api/v1/admin/users` | Admin | Scaffold; hiện trả danh sách cố định `alice`, `bob`, `charlie` |

Các route cần JWT gửi header:

```http
Authorization: Bearer <access_token>
```

Gateway kiểm tra access token ký HS256. Token hiện được Auth Service phát với role `user`. Policy admin tồn tại nhưng chưa có API/luồng hiện hành để cấp role admin.

### Health check

```http
GET /api/v1/health
```

Response `200`:

```json
{"status":"ok"}
```

### Đăng ký

```http
POST /api/v1/auth/register
Content-Type: application/json
```

```json
{
  "username": "mai",
  "display_name": "Mai Nguyen",
  "email": "mai@example.com",
  "password": "correct-horse-battery"
}
```

`username` và `password` bắt buộc. Thành công trả `201 Created` với dạng response giống login. Lỗi chính: `400` thiếu trường/JSON sai, `409` username hoặc email đã tồn tại, `502` lỗi service phía sau, `503` User/Auth service không kết nối được. Nếu tạo user thành công nhưng bước đăng nhập sau đó lỗi, API trả `502`; tài khoản vẫn đã được tạo.

### Đăng nhập

```http
POST /api/v1/auth/login
Content-Type: application/json
```

```json
{"username":"mai","password":"correct-horse-battery"}
```

Thành công trả `200 OK`:

```json
{
  "user": {
    "id": "<user-id>",
    "username": "mai",
    "display_name": "Mai Nguyen",
    "email": "mai@example.com",
    "presence": "",
    "status_message": ""
  },
  "access_token": "<jwt>",
  "refresh_token": "<jwt>",
  "token_type": "Bearer",
  "expires_in": 7200
}
```

Lỗi chính: `400` request thiếu username/password, `401` credentials không hợp lệ, `502` Auth/User service gặp lỗi, `503` service chưa sẵn sàng.

### Làm mới token

```http
POST /api/v1/auth/refresh
Content-Type: application/json
```

```json
{"refresh_token":"<refresh-token>"}
```

Thành công trả `200 OK`:

```json
{
  "access_token": "<new-jwt>",
  "refresh_token": "<new-refresh-token>",
  "token_type": "Bearer",
  "expires_in": 7200
}
```

Refresh token được rotate; dùng token mới cho lần refresh tiếp theo. Token hết hạn, bị thu hồi hoặc không hợp lệ trả `401`. Body sai trả `400`.

### Profile của người đăng nhập

```http
GET /api/v1/profile
Authorization: Bearer <access_token>
```

Thành công trả user object có `id`, `username`, `display_name`, `email`, `presence`, `status_message`. Lỗi chính: `401` token thiếu/hỏng/hết hạn, `403` policy từ chối, `404` không tìm thấy user, `502` lỗi downstream.

Hiện `PATCH /api/v1/profile` đã có handler nhưng chưa được Casbin cho phép. Dùng `PATCH /api/v1/users/:id` với chính ID của mình để cập nhật profile:

```http
PATCH /api/v1/users/<user-id>
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "display_name": "Mai N.",
  "email": "mai@example.com",
  "status_message": "Đang online"
}
```

Các field không gửi sẽ được chuyển thành chuỗi rỗng trong request downstream theo implementation hiện tại; gửi đủ giá trị muốn giữ. User thường chỉ sửa ID khớp với subject trong JWT; sửa user khác trả `403`. Admin được Casbin cho phép theo policy.

### Xem user và presence

```http
GET /api/v1/users/<user-id>
Authorization: Bearer <access_token>
```

Trả user object. User cần JWT và policy Casbin.

```http
GET /api/v1/users/<user-id>/status
```

Không cần JWT. Response:

```json
{"user_id":"<user-id>","status":"online"}
```

Nếu không có presence key hoặc Redis lỗi, gateway trả `offline`. Presence online được gia hạn khi WebSocket còn hoạt động; khi kết nối cuối cùng đóng, key bị xóa.

### Hội thoại và lịch sử tin nhắn

`GET /api/v1/chat/conversations` trả `{ "conversations": [...] }` cho user trong JWT, gồm participant, tin nhắn gần nhất, thời gian cập nhật và số tin đang chờ giao.

Tạo/lấy hội thoại trực tiếp với bạn bè:

```http
POST /api/v1/chat/conversations
Authorization: Bearer <access_token>
Content-Type: application/json

{"recipient_id":"<user-id>"}
```

Thành công trả một conversation object. Chat Service kiểm tra recipient tồn tại, friendship đã được chấp nhận và tạo hội thoại theo cặp user id duy nhất. Lỗi trả `400` cho ID không hợp lệ, `403` nếu chưa là bạn bè, hoặc `404` nếu recipient không tồn tại.

```http
GET /api/v1/chat/conversations/<conversation-id>/messages?limit=50
Authorization: Bearer <access_token>
```

Trả `{ "messages": [...] }` theo thứ tự mới nhất trước; frontend sắp xếp theo `created_at` để hiển thị cũ trước. Limit mặc định `50`, giới hạn `1..100`. Chỉ thành viên đang hoạt động được đọc; thành viên khác nhận `403`. Khi đọc history, Gateway đánh dấu các tin nhắn do người khác gửi là delivered. Client cũng có thể báo delivered ngay khi nhận realtime bằng `POST /api/v1/chat/messages/<message-id>/delivered` với JWT.

### Gửi và đọc ảnh

Upload ảnh dùng `multipart/form-data` với `conversation_id` và file field `file` tại `POST /api/v1/chat/images`. Chỉ chấp nhận ảnh JPEG, PNG, GIF hợp lệ; giới hạn 10 MB được kiểm tra ở Gateway và Chat Service. Object được lưu trong private MinIO bucket; metadata, message ảnh và attachment được ghi vào Postgres.

Attachment URL yêu cầu JWT tại `GET /api/v1/media/:id` và chỉ đọc được bởi thành viên hội thoại chứa ảnh. Bucket MinIO không được public.

### Tin nhắn chưa nhận

```http
GET /api/v1/chat/unread?limit=50
Authorization: Bearer <access_token>
```

`limit` tùy chọn, mặc định `50`, giới hạn `1..100`. Response dự kiến:

```json
{
  "messages": [
    {
      "message_id": "<message-id>",
      "conversation_id": "<conversation-id>",
      "sender_id": "<sender-id>",
      "body": "Xin chào",
      "created_at": "<timestamp>"
    }
  ]
}
```

Gateway gọi `ListUnreadMessages`, sau đó gọi `MarkDelivered` cho từng kết quả trước khi trả response. `limit` sai trả `400`; lỗi Chat Service trả `502`. Chat Service lưu dữ liệu bền vững trong Postgres. Các tin gửi cho user offline vẫn nằm trong delivery inbox; frontend tự gọi endpoint khi WebSocket kết nối lại và đọc tiếp theo từng lô tối đa 100 tin cho đến khi hết.

## WebSocket presence và gửi tin

Kết nối bằng access token trong query (phù hợp với WebSocket browser API):

```javascript
const ws = new WebSocket(
  `ws://localhost:8080/api/v1/ws/presence?token=${encodeURIComponent(accessToken)}`
);
```

Cũng có thể gửi `Authorization: Bearer <access_token>` trong HTTP upgrade request từ client hỗ trợ custom headers. Token thiếu hoặc không hợp lệ trả HTTP `401`; role không được Casbin cho phép trả `403` trước khi upgrade.

Gateway hỗ trợ nhận JSON frame gửi tin:

```json
{
  "type": "send_message",
  "conversation_id": "<conversation-id>",
  "body": "Xin chào"
}
```

Khi Chat Service nhận và chấp nhận, gateway gửi ack:

```json
{"type":"message.accepted","message_id":"<message-id>"}
```

Lỗi JSON/schema hoặc lỗi downstream gửi:

```json
{"type":"error","error":"message could not be sent"}
```

Gateway lấy `sender_id` từ JWT, không nhận sender ID từ client. Frame khác `send_message`, thiếu conversation/body, hoặc lỗi Chat Service sẽ bị từ chối. Mỗi user có thể giữ nhiều WebSocket connections trên cùng gateway; presence được làm mới định kỳ. Gateway subscribe Redis channel `chat:user:<user-id>` cho các event và forward payload nguyên trạng tới các kết nối của user đó. Event có dạng `{"type":"message","message":{"id":"...","conversation_id":"...","sender_id":"...","body":"...","created_at":"..."}}`. Client đánh dấu delivered qua endpoint riêng sau khi nhận event.

Chat Service xác thực sender là thành viên đang hoạt động của hội thoại, commit tin nhắn và bản ghi delivery trong Postgres rồi publish event lên `chat:user:<recipient-id>`. Mọi gateway subscribe theo user đang kết nối và chuyển event tới WebSocket cục bộ. Pub/Sub dùng cho realtime; endpoint unread khôi phục tin khi client offline hoặc bỏ lỡ event. Khi Redis không có presence key của người nhận, Chat Service đồng thời publish JSON bền vững vào RabbitMQ queue `notification` để worker push/email xử lý. Hiện chưa có worker FCM/APNs/email trong project.

Call signaling dùng envelope `{ "type": string, "payload": object, "ts": number }` với các event `call.invite`, `call.accept`, `call.reject`, `call.offer`, `call.answer`, `call.ice` và `call.end`. Server relay event qua Redis channel `call:user:<user-id>`. `call.invite` gồm `callId`, `conversationId`, `callType` (`audio` hoặc `video`); sender ID do Gateway lấy từ JWT và thêm dưới `from`. Gateway kiểm tra membership hội thoại, friendship đã chấp nhận và presence online của người nhận; mỗi user chỉ có một call đồng thời. Client giữ session bằng `call.heartbeat` mỗi 12 giây; Redis session lease là 45 giây. SDP/ICE chỉ relay, không lưu DB. Audio/video đi peer-to-peer; production cần STUN/TURN endpoint truy cập được từ browser.

## Error response

REST lỗi thường có dạng:

```json
{"error":"<message>"}
```

Các status thường gặp:

- `400 Bad Request`: JSON sai hoặc tham số không hợp lệ.
- `401 Unauthorized`: thiếu/hỏng/hết hạn access token hoặc credentials sai.
- `403 Forbidden`: Casbin policy hoặc kiểm tra quyền sở hữu từ chối.
- `404 Not Found`: user không tồn tại.
- `409 Conflict`: username/email đã tồn tại.
- `502 Bad Gateway`: upstream gRPC gặp lỗi.
- `503 Service Unavailable`: upstream client chưa khởi tạo.

## Internal gRPC APIs

Các service dùng package `chatter.v1`; proto source nằm trong `proto/`. RPC chỉ dành cho service-to-service, không phải REST endpoint public.

### AuthService (`auth-service:9001`)

- `Login(LoginRequest{username,password}) -> LoginResponse{user_id,access_token,refresh_token}`
- `Refresh(RefreshRequest{refresh_token}) -> RefreshResponse{access_token,refresh_token}`
- `ValidateToken(ValidateTokenRequest{access_token}) -> ValidateTokenResponse{user_id,role,valid}`

Auth gọi User Service để xác minh password. Access token hiện có TTL 2 giờ; refresh token TTL 7 ngày, lưu Redis và rotate mỗi lần refresh.

### UserService (`user-service:9005`)

- `Register(RegisterUserRequest{username,display_name,email,password}) -> User`
- `GetUserByID(GetUserByIDRequest{user_id}) -> User`
- `UpdateUser(UpdateUserRequest{user_id,display_name,email,status_message}) -> User`
- `ValidateCredentials(ValidateCredentialsRequest{username,password}) -> User`

`User` gồm `id`, `username`, `display_name`, `email`, `presence`, `status_message`. Password không được trả trong response.

### ChatService (`chat-service:9003`)

Proto khai báo các RPC:

- `SendMessage(SendMessageRequest{conversation_id,sender_id,body}) -> SendMessageResponse{message_id,success}`
- `SendImageMessage(SendImageMessageRequest{conversation_id,sender_id,media_id,storage_key,media_type,byte_size}) -> SendImageMessageResponse{message}`
- `GetMediaObject(GetMediaObjectRequest{media_id,user_id}) -> GetMediaObjectResponse{storage_key,media_type,byte_size}`
- `GetOrCreateDirectConversation(GetOrCreateDirectConversationRequest{user_id,recipient_id}) -> Conversation`
- `ListConversations(ListConversationsRequest{user_id}) -> ListConversationsResponse{conversations}`
- `ListMessages(ListMessagesRequest{conversation_id,user_id,limit}) -> ListMessagesResponse{messages}`
- `ListUnreadMessages(ListUnreadMessagesRequest{user_id,limit}) -> ListUnreadMessagesResponse{messages}`
- `MarkDelivered(MarkDeliveredRequest{user_id,message_id}) -> MarkDeliveredResponse{success}`

Các RPC được triển khai để gửi tin, liệt kê message ID, đọc unread inbox và đánh dấu giao nhận. `ListMessages` hiện chỉ trả ID tin nhắn và chưa đủ thông tin caller để làm API lịch sử an toàn; Gateway chưa expose RPC này. Conversation list/create và full history chưa có trong proto/API.

### FriendService và Call signaling

FriendService cung cấp lời mời kết bạn, danh sách bạn, tìm kiếm user và kiểm tra friendship qua gRPC. Call signaling hiện được Gateway relay qua WebSocket/Redis; `call-service` chưa ghi lịch sử vào bảng `calls`.
