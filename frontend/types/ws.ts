import type { PresenceStatus } from '@/types/presence';
import type { ChatMessage } from '@/types/chat';

export type CallType = 'audio' | 'video';

export type CallSignalEvent =
  | { type: 'call.invite'; payload: { callId: string; conversationId: string; callType: CallType; from: string }; ts: number }
  | { type: 'call.accept'; payload: { callId: string }; ts: number }
  | { type: 'call.reject'; payload: { callId: string; reason?: string }; ts: number }
  | { type: 'call.offer'; payload: { callId: string; sdp: RTCSessionDescriptionInit; iceRestart?: boolean }; ts: number }
  | { type: 'call.answer'; payload: { callId: string; sdp: RTCSessionDescriptionInit }; ts: number }
  | { type: 'call.ice'; payload: { callId: string; candidate: RTCIceCandidateInit }; ts: number }
  | { type: 'call.end'; payload: { callId: string; reason?: string }; ts: number }
  | { type: 'call.error'; payload: { callId: string; error: string }; ts: number };

export type CallClientEvent =
  | { type: 'call.invite'; payload: { callId: string; conversationId: string; callType: CallType } }
  | { type: 'call.accept'; payload: { callId: string } }
  | { type: 'call.reject'; payload: { callId: string; reason: string } }
  | { type: 'call.offer'; payload: { callId: string; sdp: RTCSessionDescriptionInit; iceRestart?: boolean } }
  | { type: 'call.answer'; payload: { callId: string; sdp: RTCSessionDescriptionInit } }
  | { type: 'call.ice'; payload: { callId: string; candidate: RTCIceCandidateInit } }
  | { type: 'call.end'; payload: { callId: string; reason: string } }
  | { type: 'call.heartbeat'; payload: { callId: string } };

export type PresenceEvent = {
  type: 'presence';
  payload: {
    user_id: string;
    status: Exclude<PresenceStatus, 'unknown'>;
    version: number;
  };
  ts: number;
};

export type SocketEvent =
  | PresenceEvent
  | { type: 'message'; message: ChatMessage }
  | { type: 'message.accepted'; message_id: string }
  | { type: 'friend.request' | 'friend.accepted' }
  | CallSignalEvent
  | { type: 'error'; error?: string };
