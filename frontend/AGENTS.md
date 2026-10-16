# Frontend (Next.js + shadcn/ui + Tailwind)

## Stack bắt buộc
- Next.js App Router + TypeScript (không dùng Pages Router)
- Tailwind CSS cho mọi styling (không CSS module, không styled-components)
- shadcn/ui làm component nền; thêm component bằng `npx shadcn@latest add <name>`
- Zustand: global client state (modal, current user, call state)
- TanStack Query: mọi data fetching/caching/infinite scroll
- react-hook-form + zod: mọi form
- WebSocket và WebRTC dùng API native của trình duyệt

## Phiên bản Next.js
Kiểm tra `package.json` trước khi viết code liên quan routing/middleware:
- **Next.js >= 16**: dùng `proxy.ts` (export function `proxy`) thay cho `middleware.ts`;
  `params`, `searchParams`, `cookies()`, `headers()` là **async** (phải `await`).
- **Next.js < 16** (vd 14.x): dùng `middleware.ts`; `params` đồng bộ.
- Nếu repo còn ở bản cũ và có cảnh báo bảo mật từ `npm audit`, đề xuất nâng cấp lên 16
  trên **branch riêng** (`npx @next/codemod@canary upgrade latest`), không tự nâng khi chưa được yêu cầu.

## Cấu trúc thư mục (không tự đổi tên thư mục gốc)
```
app/(auth)/login/page.tsx
app/(auth)/register/page.tsx
app/(main)/chat/layout.tsx              # layout 3 cột
app/(main)/chat/page.tsx                # empty state
app/(main)/chat/[conversationId]/page.tsx
components/ui/                          # shadcn sinh ra, không sửa tay
components/chat/  components/friend/  components/call/
hooks/   lib/   types/   stores/
```
- Component dùng chung KHÔNG đặt trong `app/`.
- `"use client"` chỉ thêm cho component có state/event; component thuần hiển thị giữ là Server Component.

## Quy tắc code
- Cấm `any`. Type nằm trong `types/*.ts`, khớp proto/OpenAPI backend.
- Mọi API call đi qua `lib/api-client.ts` (có refresh token). Không gọi `fetch` trực tiếp trong component.
- Chỉ **một** WebSocket connection cho toàn app, quản lý trong `hooks/use-websocket.ts`.
- Danh sách phải có loading state (shadcn `Skeleton`); lỗi API phải có toast (Sonner). Không fail âm thầm.
- Không tự nghĩ ra màu/spacing mới: dùng token của Tailwind + theme shadcn. Hỗ trợ dark mode (next-themes).
- Token: ưu tiên cookie httpOnly hoặc memory, không lưu access token vào localStorage.

## Luật cứng về UI
- Kết bạn **luôn là Dialog** (`FriendModal`), KHÔNG tạo route `/friends`.
- Layout chat là 3 cột trên desktop; mobile ẩn cột giữa khi đang xem một conversation, có nút back.
- Logic WebRTC nằm trong hook `useWebRTC`; component UI không tự quản lý `RTCPeerConnection`.

## Trước khi tự viết component
1. Kiểm tra shadcn đã có component phù hợp chưa.
2. Với UI chat (bubble, input, list) tham khảo `shadcn-chat` thay vì tự dựng từ đầu.
3. Dùng dữ liệu mock (`lib/mock/`) khi backend chưa sẵn sàng, tách biệt hẳn khỏi code thật.

## Kiểm chứng
`npm run lint && npx tsc --noEmit && npm run build`
