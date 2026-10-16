# Backend (Golang + gRPC)

## Stack
- Go (module-based), gRPC + protobuf, grpc-gateway cho REST/JSON
- Postgres (nguồn sự thật), Redis (cache, pub/sub, presence), RabbitMQ (tác vụ bất đồng bộ)
- Gợi ý thư viện: `pgx` (Postgres), `golang-migrate` (migration), `go-redis`, `amqp091-go`,
  `gorilla/websocket` hoặc `coder/websocket`. Không thêm thư viện khác khi chưa nêu lý do.

## Cấu trúc
```
backend/
  cmd/server/main.go            # entrypoint, wiring
  internal/
    auth/  friend/  chat/  call/ # mỗi module: handler, service, repository, model
    gateway/                     # HTTP/WebSocket gateway, middleware JWT
    platform/                    # postgres, redis, rabbitmq, config, logger
  proto/                         # *.proto (source of truth)
  migrations/                    # *.up.sql / *.down.sql
```
- Module giao tiếp qua interface/function call; chỉ tách gRPC service riêng khi được yêu cầu.
- Phụ thuộc theo chiều: handler → service → repository. Không import ngược.

## Quy tắc code
- `context.Context` là tham số đầu tiên cho mọi hàm I/O. Luôn truyền timeout cho call ra ngoài.
- Lỗi: wrap bằng `fmt.Errorf("...: %w", err)`; map sang gRPC status code ở tầng handler.
- Không dùng global state; inject dependency qua constructor.
- Test dạng **table-driven**; repository test dùng Postgres thật (docker/testcontainers), không mock SQL.
- Mọi thay đổi schema phải qua migration có cả `up` và `down`. Không sửa migration đã merge.
- `gofmt` và `go vet` phải sạch.

## Quy ước dữ liệu
- Postgres: schema theo `docs/db` hoặc thiết kế đã thống nhất. Dùng `BIGSERIAL` cho id; `TIMESTAMPTZ` cho thời gian.
- `friendships` lưu một dòng với `user_id_1 < user_id_2` (CHECK constraint).
- Chấp nhận kết bạn phải nằm trong **một transaction** (update request + insert friendship).
- Index bắt buộc: `messages(conversation_id, created_at DESC)`, `conversation_members(user_id)`.
- Unread count dùng `conversation_members.last_read_message_id`, không đếm qua bảng receipts.
- Redis key: `user:status:{userID}` (TTL ngắn, heartbeat), `friends:{userID}` (cache bạn bè),
  channel pub/sub: `chat:user:{userID}`. Không lưu online status vào Postgres.
- RabbitMQ: exchange/queue đặt tên theo `<domain>.<event>` (vd `friend.request_created`), consumer idempotent.
- SDP/ICE **không lưu DB**; chỉ relay qua signaling.

## Bảo mật
- Mật khẩu hash bằng bcrypt/argon2. JWT access ngắn hạn + refresh token (lưu hash).
- Kiểm tra quyền ở service layer: chỉ bạn bè mới được nhắn tin 1-1/gọi (`IsFriend`); chỉ member mới đọc được conversation.
- Giới hạn tốc độ cho đăng nhập, gửi lời mời kết bạn, gửi tin nhắn.

## Kiểm chứng
`gofmt -l . && go vet ./... && go test ./...`
