# Chatter – Codex harness

Giải nén vào gốc repo `~/chatter` (giữ nguyên cấu trúc thư mục):

```
chatter/
├── AGENTS.md
├── .codex/config.toml
├── .agents/skills/{messenger-frontend,friend-modal,webrtc-call,go-service}/SKILL.md
├── frontend/AGENTS.md
└── backend/AGENTS.md
```

## Cách dùng
1. Mở terminal **WSL**, đảm bảo `which node` trỏ về `~/.nvm/...`.
2. `cd ~/chatter && codex` (lần đầu chọn "trust" project để `.codex/config.toml` có hiệu lực).
3. Gõ `/skills` để xem các skill đã nhận; có thể gọi rõ tên skill trong prompt.
4. Làm từng task nhỏ theo thứ tự gợi ý:
   login/register → proxy/middleware auth → layout 3 cột (mock) → FriendModal → nối API/WebSocket → WebRTC.

## Prompt mẫu
```
Đọc skill friend-modal. Đề xuất kế hoạch (chưa sửa file) để xây FriendModal
với dữ liệu mock, mở qua Zustand store. Sau khi tôi duyệt mới thực thi.
```
