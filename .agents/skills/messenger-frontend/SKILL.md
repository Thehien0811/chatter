---
name: messenger-frontend
description: Dùng khi tạo hoặc sửa UI frontend của Chatter - layout 3 cột kiểu Messenger, danh sách hội thoại, chat bubble, ô nhập tin nhắn, WebSocket client, state và query. Không dùng cho Friend modal hay video call (có skill riêng).
---

# Skill: Messenger Frontend

## Mục tiêu
Dựng giao diện chat kiểu Messenger sau khi đăng nhập, bằng Next.js + shadcn/ui + Tailwind.

## Quy trình
1. Đọc `frontend/AGENTS.md` để nắm stack và luật cứng.
2. Kiểm tra component shadcn đã có chưa trước khi tự viết. Tham khảo `shadcn-chat` cho bubble/input.
3. Định nghĩa/đối chiếu type trong `types/` trước khi viết UI.
4. Dùng dữ liệu mock cho tới khi API thật sẵn sàng.
5. Chạy `npm run lint && npx tsc --noEmit && npm run build` trước khi báo xong.

## Layout `/chat`
- 3 cột desktop: **sidebar icon (~72px)** | **danh sách hội thoại (~320px)** | **khung chat (flex-1)**.
- Sidebar icon: icon Chat, icon Lời mời kết bạn (badge số pending, mở FriendModal), avatar user ở dưới cùng (dropdown: Profile, Settings, Logout).
- Danh sách hội thoại: ô tìm kiếm, item gồm avatar, tên, tin cuối (truncate), thời gian, badge chưa đọc; sort theo `updated_at` giảm dần; item active có nền khác biệt; dùng `ScrollArea`.
- Khung chat: header (avatar, tên, chấm online, nút gọi video/thoại, menu), vùng tin nhắn, ô nhập.
- Mobile (<768px): ẩn cột giữa khi đã chọn hội thoại, có nút back.
- Chưa chọn hội thoại: empty state "Chọn một cuộc trò chuyện để bắt đầu".

## Chat UI
- Tin của mình: căn phải, màu `primary`. Tin người khác: căn trái, màu `muted`, kèm avatar.
- Gộp tin liên tiếp cùng người gửi trong vài phút: không lặp avatar/tên.
- Trạng thái tin: icon sent / delivered / seen (không dùng text).
- Typing indicator "đang nhập..." khi có event `typing`.
- Ô nhập: textarea tự giãn, Enter gửi, Shift+Enter xuống dòng, nút đính kèm, emoji, gửi.
- Tự cuộn xuống cuối khi có tin mới (trừ khi người dùng đang đọc tin cũ). Tải tin cũ khi kéo lên bằng `useInfiniteQuery`.
- Gửi tin kiểu optimistic: hiển thị ngay với trạng thái `sending`, cập nhật khi server ack, báo lỗi + nút thử lại nếu thất bại.

## Data & state
- Query keys: `["conversations"]`, `["messages", conversationId]`, `["friends"]`, `["friend-requests"]`.
- Event WebSocket cập nhật cache của TanStack Query (`queryClient.setQueryData`), không refetch toàn bộ.
- Zustand chỉ giữ state UI/phiên: `currentUser`, `activeConversationId`, `friendModal`, `call`.

## Hợp đồng WebSocket
Envelope: `{ "type": string, "payload": object, "ts": number }`. Định nghĩa trong `types/ws.ts` dạng discriminated union.

| type | Hướng | Ý nghĩa |
|---|---|---|
| `message.send` | client → server | Gửi tin nhắn |
| `message.new` | server → client | Có tin nhắn mới |
| `message.status` | server → client | Cập nhật sent/delivered/seen |
| `message.read` | client → server | Đánh dấu đã đọc tới message id |
| `typing` | hai chiều | Đang nhập |
| `presence` | server → client | Online/offline của bạn bè |
| `friend.request` | server → client | Nhận lời mời kết bạn |
| `friend.accepted` | server → client | Lời mời được chấp nhận |
| `call.*` | hai chiều | Signaling (xem skill `webrtc-call`) |

Quy tắc client: một connection duy nhất; tự reconnect với exponential backoff; gửi heartbeat định kỳ; sau khi reconnect thì refetch `["conversations"]` và tin nhắn của hội thoại đang mở.

## Việc cần tránh
- Tạo nhiều WebSocket connection ở nhiều component.
- Gọi `fetch` trực tiếp trong component.
- Dùng `any`, hoặc đoán field backend khi chưa có proto/OpenAPI.
- Tự đặt màu/spacing ngoài token của theme.
