'use client';
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import { ActiveCallScreen } from '@/components/call/active-call-screen';
import { IncomingCallDialog } from '@/components/call/incoming-call-dialog';
import { api } from '@/lib/api-client';
import { useWebRTC, type CallInfo } from '@/hooks/use-webrtc';
import type { User } from '@/types/user';

type CallContextValue = {
  startAudioCall: (conversationId: string, peerId: string, peerName: string) => Promise<void>;
};

const CallContext = createContext<CallContextValue | null>(null);

export function useCallManager() {
  const context = useContext(CallContext);
  if (!context) throw new Error('useCallManager must be used within CallManager.');
  return context;
}

export function CallManager({ children }: { children: ReactNode }) {
  const call = useWebRTC();
  const [incomingPeerName, setIncomingPeerName] = useState<{ id: string; name: string } | null>(null);
  const [outgoingPeerName, setOutgoingPeerName] = useState<{ id: string; name: string } | null>(null);
  const currentCall: CallInfo | null = call.call;

  useEffect(() => {
    if (!currentCall || currentCall.direction !== 'incoming' || !currentCall.peerId) return;
    let live = true;
    api<User>(`/users/${encodeURIComponent(currentCall.peerId)}`)
      .then((user) => { if (live) setIncomingPeerName({ id: currentCall.peerId, name: user.display_name || user.username }); })
      .catch((error: unknown) => {
        if (!live) return;
        setIncomingPeerName({ id: currentCall.peerId, name: currentCall.peerId });
        toast.error(error instanceof Error ? error.message : 'Không tải được thông tin người gọi.');
      });
    return () => { live = false; };
  }, [currentCall]);

  const startWebRTCCall = call.startCall;
  const value = useMemo<CallContextValue>(() => ({
    startAudioCall: async (conversationId, peerId, name) => {
      setOutgoingPeerName({ id: peerId, name: name || peerId });
      try {
        await startWebRTCCall(conversationId, 'audio', peerId);
      } catch (error) {
        throw error;
      }
    },
  }), [startWebRTCCall]);

  const showCallScreen = call.status !== 'idle' && call.status !== 'incoming';
  const displayName = currentCall
    ? currentCall.direction === 'incoming'
      ? incomingPeerName?.id === currentCall.peerId ? incomingPeerName.name : currentCall.peerId
      : outgoingPeerName?.id === currentCall.peerId ? outgoingPeerName.name : 'Người nhận'
    : 'Cuộc gọi';
  return <CallContext.Provider value={value}>
    {children}
    <IncomingCallDialog
      open={call.status === 'incoming'}
      callerName={displayName || 'Người gọi'}
      video={currentCall?.callType === 'video'}
      onAccept={() => { void call.acceptCall(); }}
      onDecline={call.rejectCall}
    />
    <ActiveCallScreen
      key={currentCall?.id ?? 'no-call'}
      open={showCallScreen}
      call={currentCall}
      callerName={displayName}
      status={call.status}
      localStream={call.localStream}
      remoteStream={call.remoteStream}
      isMuted={call.isMuted}
      error={call.error}
      endReason={call.endReason}
      onToggleMute={call.toggleMute}
      onEnd={call.endCall}
    />
  </CallContext.Provider>;
}
