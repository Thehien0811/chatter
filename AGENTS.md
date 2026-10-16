# Chatter – ứng dụng chat kiểu Messenger

Ba chức năng chính: **nhắn tin**, **kết bạn**, **gọi video**.

## Kiến trúc tổng quan
- `frontend/`: Next.js (App Router, TypeScript) + shadcn/ui + Tailwind CSS
- `backend/`: Golang, gRPC + gRPC-Gateway (REST/JSON cho frontend), Postgres, Redis, RabbitMQ
- Realtime: WebSocket qua API Gateway. Video call: WebRTC (media P2P, signaling qua WebSocket)
- Bắt đầu theo hướng **monolith chia module** (`internal/auth|friend|chat|call`), chỉ tách microservice khi thật sự cần.

## Môi trường (QUAN TRỌNG)
- Chạy MỌI lệnh trong **WSL (Ubuntu)** với Node cài bằng **nvm**. Không dùng Node của Windows.
- Trước khi làm việc kiểm tra: `which node` phải trỏ về `/home/<user>/.nvm/...`, KHÔNG phải `/mnt/c/...`.
- Code nằm trong filesystem WSL (`~/chatter`), không đặt trong `/mnt/c`.

## Lệnh kiểm chứng (bắt buộc chạy trước khi báo hoàn thành)
- Frontend: `cd frontend && npm run lint && npx tsc --noEmit && npm run build`
- Backend: `cd backend && gofmt -l . && go vet ./... && go test ./...`

## Quy tắc làm việc
1. Mỗi task chỉ một mục tiêu nhỏ, diff đủ nhỏ để review. Không refactor ngoài phạm vi yêu cầu.
2. Với task lớn: **đề xuất kế hoạch trước, chưa sửa file**, đợi xác nhận rồi mới code.
3. Không tự thêm dependency mới khi chưa nêu lý do và được đồng ý.
4. Không sửa: `node_modules/`, `.next/`, file sinh tự động từ proto (`*.pb.go`, `*_grpc.pb.go`, `*.pb.gw.go`).
5. Không hard-code secret. Dùng biến môi trường và cập nhật `.env.example`.
6. Không đụng backend và frontend trong cùng một task, trừ khi task là "đồng bộ hợp đồng API".
7. Commit theo Conventional Commits (`feat:`, `fix:`, `chore:`, `refactor:`, `test:`, `docs:`).
8. Khi không chắc về hợp đồng API/proto, **hỏi lại**, không tự đoán field.

## Hợp đồng API (source of truth)
- Backend: `backend/proto/*.proto` là nguồn sự thật. Frontend type phải khớp với đây.
- Realtime dùng chung envelope JSON: `{ "type": string, "payload": object, "ts": number }`.
  Chi tiết các `type` xem skill `messenger-frontend`.

## Definition of Done
- Lint, typecheck, build/test đều pass.
- Có mô tả ngắn: đã đổi gì, cách tự kiểm tra thủ công, việc còn dang dở (nếu có).

## Skills có sẵn (`.agents/skills/`)
- `messenger-frontend`: layout 3 cột, chat UI, WebSocket client, state/query
- `friend-modal`: chức năng kết bạn dạng Dialog
- `webrtc-call`: gọi video, signaling, hook `useWebRTC`
- `go-service`: thêm tính năng backend theo chuẩn của dự án
