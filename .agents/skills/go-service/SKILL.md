---
name: go-service
description: Dùng khi thêm hoặc sửa tính năng backend Golang của Chatter (auth, friend, chat, call) - quy trình từ proto, migration, repository, service, handler đến test. Dùng khi cần tạo endpoint mới hoặc thay đổi schema.
---

# Skill: Thêm tính năng backend (Go)

## Quy trình chuẩn (đi theo thứ tự, mỗi bước nhỏ)
1. **Làm rõ yêu cầu & hợp đồng**: viết/sửa `proto/<module>.proto` (message + rpc + HTTP annotation cho gateway). Hỏi lại nếu mơ hồ.
2. **Sinh code** từ proto (dùng lệnh/Makefile của dự án). Không sửa file sinh tự động.
3. **Migration**: tạo cặp `migrations/NNNN_<tên>.up.sql` và `.down.sql`. Thêm index cần thiết. Không sửa migration cũ.
4. **Repository** (`internal/<module>/repository.go`): SQL thuần với `pgx`, tham số hóa, nhận `context.Context`. Interface định nghĩa ở nơi dùng.
5. **Service** (`internal/<module>/service.go`): nghiệp vụ, kiểm tra quyền, mở transaction khi cần ghi nhiều bảng. Không phụ thuộc gRPC/HTTP.
6. **Handler** (`internal/<module>/handler.go`): map request ↔ domain, map lỗi → gRPC status code.
7. **Wiring** trong `cmd/server/main.go`.
8. **Test**: table-driven cho service; repository test với Postgres thật. Chạy `gofmt -l . && go vet ./... && go test ./...`.

## Quy ước lỗi (map sang gRPC code)
| Tình huống | Code |
|---|---|
| Input sai | `InvalidArgument` |
| Chưa đăng nhập / token hỏng | `Unauthenticated` |
| Không đủ quyền (không phải bạn bè/member) | `PermissionDenied` |
| Không tìm thấy | `NotFound` |
| Trùng (vd đã gửi lời mời) | `AlreadyExists` |
| Lỗi hệ thống | `Internal` (log chi tiết, không lộ ra client) |

## Mẫu theo module
- **friend**: `SendRequest`, `AcceptRequest` (transaction: update request + insert friendship với `user_id_1 < user_id_2`), `RejectRequest`, `ListFriends` (cache Redis `friends:{id}`), `IsFriend(a, b)`.
- **chat**: `SendMessage` (lưu Postgres → publish Redis `chat:user:{id}`; nếu người nhận offline → publish RabbitMQ `chat.message_offline`), `ListMessages` (phân trang theo cursor `id`/`created_at`), `MarkRead` (cập nhật `last_read_message_id`).
- **call**: kiểm tra `IsFriend` và trạng thái online (Redis) trước khi relay `invite`; lưu lịch sử vào bảng `calls`; SDP/ICE chỉ relay, không lưu.
- **auth**: đăng ký/đăng nhập, bcrypt/argon2, JWT access ngắn hạn + refresh token (lưu hash, có thể thu hồi).

## Checklist trước khi báo xong
- [ ] Proto và code sinh đồng bộ, không sửa tay file sinh.
- [ ] Migration có `up` + `down`, có index phù hợp.
- [ ] Có kiểm tra quyền ở service layer.
- [ ] Mọi hàm I/O nhận `context.Context`, có timeout cho call ngoài.
- [ ] Test mới pass, `go vet` và `gofmt` sạch.
- [ ] Cập nhật `.env.example` nếu thêm biến cấu hình.
