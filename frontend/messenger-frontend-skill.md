# Skill: Messenger Frontend (Next.js + shadcn/ui + Tailwind)

## Khi nào dùng skill này
Áp dụng skill này khi cần tạo mới hoặc chỉnh sửa code frontend cho ứng dụng
chat kiểu Messenger, với backend Golang/gRPC-Gateway, WebSocket cho realtime,
WebRTC cho video call. Dùng cho mọi task: tạo trang mới, tạo component,
sửa layout, thêm feature, viết hook.

## Tech stack bắt buộc
- Next.js 14+ (App Router, TypeScript, không dùng Pages Router)
- Tailwind CSS cho toàn bộ styling — không viết CSS module hay styled-components
- shadcn/ui làm base component library (Button, Input, Dialog, Avatar,
  ScrollArea, Tabs, DropdownMenu, Badge, Sonner, Skeleton)
- Zustand cho global client state (modal state, current user, call state)
- TanStack Query (React Query) cho toàn bộ data fetching, caching,
  infinite scroll — không tự viết useEffect + fetch thủ công
- react-hook-form + zod cho mọi form và validation
- native WebSocket API cho realtime message/notification
- native WebRTC API cho video call (RTCPeerConnection)

## Quy tắc cấu trúc thư mục
Luôn tuân theo cấu trúc sau, không tự ý đổi tên thư mục gốc:

```
app/
  (auth)/login/page.tsx
  (auth)/register/page.tsx
  (main)/chat/layout.tsx          # 3-column layout
  (main)/chat/page.tsx             # empty state
  (main)/chat/[conversationId]/page.tsx
components/
  ui/                              # shadcn generated components, không sửa tay
  chat/conversation-list.tsx
  chat/message-bubble.tsx
  chat/message-input.tsx
  chat/chat-header.tsx
  friend/friend-modal.tsx
  friend/friend-request-item.tsx
  call/incoming-call-dialog.tsx
  call/call-screen.tsx
hooks/
  use-websocket.ts
  use-webrtc.ts
  use-friend-modal-store.ts
lib/
  api-client.ts                    # fetch wrapper + refresh token interceptor
  websocket-client.ts
types/
  message.ts
  conversation.ts
  user.ts
  call.ts
```

Khi tạo file mới, đặt đúng vào thư mục tương ứng ở trên. Không tạo component
dùng chung trong `app/`.

## Quy tắc UI / UX bắt buộc

### Layout chính (`/chat`)
- 3 cột cố định: sidebar icon (~72px) → conversation list (~320px) →
  khung chat (flex-1).
- Trên mobile (< 768px): ẩn cột giữa khi đang mở 1 conversation, có nút back
  quay lại danh sách. Không dùng 3 cột song song trên mobile.
- Conversation đang active phải có background khác biệt rõ với các item khác.

### Chat bubble
- Tin nhắn của mình: căn phải, dùng màu `primary`.
- Tin nhắn người khác: căn trái, dùng màu `muted`, kèm avatar.
- Gộp các tin nhắn liên tiếp cùng người gửi trong vòng vài phút, không lặp
  avatar/tên cho từng tin.
- Luôn hiển thị trạng thái sent/delivered/seen bằng icon, không dùng text.

### Friend feature — LUÔN là Dialog, không phải route riêng
- Không được tạo `app/friends/page.tsx` hay route độc lập cho tính năng bạn
  bè. Toàn bộ UI kết bạn nằm trong 1 component `<FriendModal />` dùng
  shadcn `Dialog` + `Tabs` (3 tab: Thêm bạn / Lời mời đã nhận / Danh sách bạn bè).
- Trạng thái mở/đóng modal và tab active quản lý qua Zustand store
  `useFriendModalStore`, để bất kỳ component nào (sidebar icon, nút "+" ở
  conversation list...) cũng gọi mở modal được mà không cần prop drilling.
- Chấp nhận lời mời xong: đóng modal, tạo/mở conversation 1-1, điều hướng
  sang `/chat/[conversationId]`.

### Video call
- Cuộc gọi đến: overlay/Dialog toàn màn hình, không phải toast hay banner nhỏ.
- Trong cuộc gọi: video local nhỏ góc dưới phải (draggable), video remote
  full màn hình, thanh control ở dưới cùng (mute, tắt cam, kết thúc — nút đỏ).
- Logic WebRTC tách hoàn toàn khỏi UI, nằm trong hook `useWebRTC()`. Component
  UI chỉ gọi hook, không tự quản lý `RTCPeerConnection`.

## Quy tắc code
- Mọi component tương tác (có state, event handler) phải có `"use client"`
  ở đầu file; component thuần hiển thị nên là Server Component khi có thể.
- Không dùng `any`; định nghĩa type trong `types/*.ts` khớp với response
  backend thật (dựa theo proto/OpenAPI nếu có), không tự đoán field.
- Mọi API call đi qua `lib/api-client.ts` (có sẵn interceptor refresh token),
  không gọi `fetch` trực tiếp trong component.
- WebSocket chỉ có 1 connection dùng chung toàn app, quản lý qua
  `hooks/use-websocket.ts`, không tạo nhiều connection ở nhiều component.
- Luôn thêm loading state bằng shadcn `Skeleton` cho danh sách (conversation
  list, message list) khi query đang pending — không để trắng màn hình.
- Luôn thêm error toast (Sonner) khi API call thất bại — không fail âm thầm.

## Việc cần làm trước khi code UI mới
1. Kiểm tra xem shadcn đã có component phù hợp chưa (`npx shadcn add ...`)
   trước khi tự viết component từ đầu.
2. Nếu cần pattern UI phức tạp hơn (chat bubble, message list), tham khảo
   thư viện `shadcn-chat` (jakobhoeg/shadcn-chat) — đã build sẵn trên nền
   shadcn/ui + Tailwind, tránh tự thiết kế sai layout chat từ đầu.
3. Không tự nghĩ ra spacing/màu sắc mới — dùng token có sẵn trong
   `tailwind.config.ts` và theme của shadcn.
