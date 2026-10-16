---
name: friend-modal
description: Dùng khi xây hoặc sửa chức năng kết bạn của Chatter. Toàn bộ UI kết bạn là một Dialog (modal) với 3 tab, mở được từ bất kỳ đâu qua Zustand store. Không tạo route riêng cho bạn bè.
---

# Skill: Friend Modal

## Luật cứng
- KHÔNG tạo `app/friends/page.tsx` hay bất kỳ route độc lập nào cho kết bạn.
- Toàn bộ UI nằm trong `components/friend/friend-modal.tsx` dùng shadcn `Dialog` + `Tabs`.

## Store (`stores/friend-modal-store.ts`)
```ts
type FriendTab = "add" | "received" | "friends";

interface FriendModalState {
  open: boolean;
  tab: FriendTab;
  openModal: (tab?: FriendTab) => void;
  closeModal: () => void;
  setTab: (tab: FriendTab) => void;
}
```
Mọi nơi (icon sidebar, nút "+" ở danh sách hội thoại, toast "có lời mời mới") chỉ gọi `openModal(tab)`. Render `<FriendModal />` **một lần** ở `app/(main)/chat/layout.tsx`.

## Ba tab
1. **Thêm bạn (`add`)**: ô tìm theo username/email (debounce ~300ms) → danh sách kết quả: avatar, tên, nút theo trạng thái
   `Gửi lời mời` / `Đã gửi` (disabled hoặc "Hủy lời mời") / `Bạn bè`. Không hiển thị chính mình.
2. **Lời mời đã nhận (`received`)**: mỗi item có `Chấp nhận` (variant default) và `Từ chối` (variant outline/ghost).
   Hiện badge đỏ số lượng trên tab và trên icon sidebar khi có lời mời pending.
3. **Danh sách bạn bè (`friends`)**: click một người → mở hoặc tạo hội thoại 1-1, đóng modal,
   chuyển sang route `/chat/{conversationId}` bằng `router.push`.

## Component
- `friend-modal.tsx` (khung + tabs), `friend-search-tab.tsx`, `friend-requests-tab.tsx`, `friend-list-tab.tsx`, `friend-request-item.tsx`.
- Mỗi tab là component riêng để dễ test và lazy fetch (chỉ query khi tab được mở).

## API (giả định, đối chiếu proto trước khi dùng)
- `GET /users/search?q=`, `POST /friend-requests`, `GET /friend-requests?direction=received`,
  `POST /friend-requests/{id}/accept`, `POST /friend-requests/{id}/reject`, `DELETE /friend-requests/{id}`, `GET /friends`.
- Nếu backend chưa có: dùng mock trong `lib/mock/friends.ts`, bọc sau cùng interface với API thật để đổi dễ.

## Data & realtime
- Query keys: `["friend-search", q]`, `["friend-requests"]`, `["friends"]`.
- Mutation dùng optimistic update; thất bại thì rollback + toast lỗi (Sonner).
- Sau khi chấp nhận: invalidate `["friends"]` và `["friend-requests"]`.
- Event `friend.request` / `friend.accepted` qua WebSocket cập nhật cache và badge; nếu modal đang đóng thì hiện toast có nút "Xem".

## Trạng thái cần xử lý
Loading (Skeleton), rỗng (text hướng dẫn), lỗi (toast + nút thử lại), đang gửi (disable nút, spinner), trùng lời mời, tự gửi cho mình.

## Tiêu chí hoàn thành
- Mở được modal từ sidebar và từ nút "+".
- Gửi, hủy, chấp nhận, từ chối đều hoạt động (mock hoặc thật) và cập nhật badge đúng.
- Chọn bạn → chuyển đúng route hội thoại, modal đóng.
- Lint, typecheck, build đều pass.
