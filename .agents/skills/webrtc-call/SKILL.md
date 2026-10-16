---
name: webrtc-call
description: Dùng khi xây hoặc sửa chức năng gọi video/thoại 1-1 của Chatter - hook useWebRTC, luồng signaling qua WebSocket, giao diện cuộc gọi đến và màn hình đang gọi, xử lý STUN/TURN. Không dùng cho gọi nhóm (cần SFU).
---

# Skill: WebRTC Call (1-1)

## Nguyên tắc
- Media đi **P2P** giữa hai trình duyệt; server chỉ **relay signaling** qua WebSocket.
- Logic WebRTC nằm hoàn toàn trong `hooks/use-webrtc.ts`. Component UI chỉ gọi hook, không tự tạo `RTCPeerConnection`.
- SDP/ICE không lưu DB, chỉ đi qua signaling.

## Thông điệp signaling (envelope chung `{type, payload, ts}`)
| type | Hướng | payload chính |
|---|---|---|
| `call.invite` | caller → server → callee | `callId`, `conversationId`, `callType` (audio/video), `from` |
| `call.accept` | callee → caller | `callId` |
| `call.reject` | callee → caller | `callId`, `reason` |
| `call.offer` | caller → callee | `callId`, `sdp` |
| `call.answer` | callee → caller | `callId`, `sdp` |
| `call.ice` | hai chiều | `callId`, `candidate` |
| `call.end` | hai chiều | `callId`, `reason` (hangup/timeout/network_error) |

Luồng: `invite` → callee `accept` → caller tạo `offer` → callee trả `answer` → hai bên trao `ice` → kết nối P2P.
Server kiểm tra callee online và là bạn bè trước khi chuyển `invite`.

## Hook `useWebRTC`
API đề xuất:
```ts
type CallStatus = "idle" | "outgoing" | "incoming" | "connecting" | "connected" | "ended";

interface UseWebRTC {
  status: CallStatus;
  localStream: MediaStream | null;
  remoteStream: MediaStream | null;
  isMuted: boolean;
  isCameraOff: boolean;
  startCall: (conversationId: string, type: "audio" | "video") => Promise<void>;
  acceptCall: () => Promise<void>;
  rejectCall: () => void;
  endCall: () => void;
  toggleMute: () => void;
  toggleCamera: () => void;
}
```
Chi tiết bắt buộc:
- Lấy media bằng `getUserMedia`; xử lý từ chối quyền, không có thiết bị → báo lỗi rõ ràng.
- Cấu hình ICE từ biến môi trường: `NEXT_PUBLIC_STUN_URL`, `NEXT_PUBLIC_TURN_URL`, `NEXT_PUBLIC_TURN_USER`, `NEXT_PUBLIC_TURN_PASS`. Luôn có TURN trong production (khoảng 15-20% người dùng không P2P được).
- Gắn `onicecandidate` → gửi `call.ice`; `ontrack` → set `remoteStream`.
- Đặt ICE candidate đến vào hàng đợi cho tới khi đã có `remoteDescription`.
- Theo dõi `connectionState`/`iceConnectionState`; `failed` → thử `restartIce()` một lần rồi mới kết thúc.
- Timeout chuông ~30s nếu không ai trả lời → `call.end` với reason `timeout`.
- **Cleanup**: `endCall` phải dừng mọi track (`track.stop()`), `pc.close()`, reset state, gỡ listener. Tránh rò rỉ camera.
- Chỉ cho một cuộc gọi tại một thời điểm; nếu đang gọi mà có invite mới → tự `reject` với reason `busy`.

## Giao diện
- **Cuộc gọi đến** (`incoming-call-dialog.tsx`): overlay/Dialog toàn màn hình, avatar + tên người gọi, nút Accept (xanh) / Decline (đỏ), chuông lặp cho tới khi xử lý hoặc timeout. Không dùng toast nhỏ.
- **Đang gọi** (`call-screen.tsx`): video remote full màn hình; video local nhỏ ở góc dưới phải, kéo thả được; thanh điều khiển dưới cùng (mute, tắt cam, kết thúc - nút đỏ, thu nhỏ).
- Với audio call: hiển thị avatar lớn thay vì video, giữ nguyên thanh điều khiển.
- Hiển thị trạng thái "Đang gọi...", "Đang kết nối...", thời lượng khi đã connected.
- Render `<IncomingCallDialog />` và `<CallScreen />` một lần ở `app/(main)/chat/layout.tsx`, điều khiển bằng state từ hook/store.

## Kiểm thử thủ công (chưa cần backend thật)
- Mở hai tab trình duyệt (hai user) hoặc dùng mock signaling bằng `BroadcastChannel` để thử luồng offer/answer.
- Thử: từ chối cuộc gọi, không bắt máy (timeout), tắt/bật mic và cam, tắt mạng giữa chừng, đóng tab khi đang gọi.

## Việc cần tránh
- Tạo `RTCPeerConnection` ngoài hook.
- Quên dừng track khi kết thúc.
- Lưu SDP/ICE vào DB hoặc localStorage.
- Làm gọi nhóm bằng mesh P2P (nếu cần nhóm, dùng SFU như LiveKit/mediasoup - việc khác).
