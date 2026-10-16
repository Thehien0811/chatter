'use client';
import { useEffect, useRef, useState } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import { Mic, MicOff, Phone, PhoneOff } from 'lucide-react';
import { Avatar } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import type { CallInfo, CallStatus } from '@/hooks/use-webrtc';

export function ActiveCallScreen({
  open,
  call,
  callerName,
  status,
  localStream,
  remoteStream,
  isMuted,
  error,
  endReason,
  onToggleMute,
  onEnd,
}: {
  open: boolean;
  call: CallInfo | null;
  callerName: string;
  status: CallStatus;
  localStream: MediaStream | null;
  remoteStream: MediaStream | null;
  isMuted: boolean;
  error: string;
  endReason: string;
  onToggleMute: () => void;
  onEnd: () => void;
}) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [duration, setDuration] = useState(0);

  useEffect(() => {
    if (audioRef.current) audioRef.current.srcObject = remoteStream;
  }, [remoteStream]);

  useEffect(() => {
    if (status !== 'connected') {
      return;
    }
    const started = Date.now();
    const timer = window.setInterval(() => setDuration(Math.floor((Date.now() - started) / 1000)), 1000);
    return () => window.clearInterval(timer);
  }, [call?.id, status]);

  if (!call) return null;
  const statusText = status === 'outgoing' ? 'Đang gọi…'
    : status === 'connecting' ? 'Đang kết nối…'
      : status === 'connected' ? `${Math.floor(duration / 60).toString().padStart(2, '0')}:${(duration % 60).toString().padStart(2, '0')}`
        : status === 'ended' ? endReason === 'timeout' ? 'Không có người trả lời' : endReason === 'declined' ? 'Cuộc gọi bị từ chối' : endReason === 'busy' ? 'Người nhận đang bận' : 'Cuộc gọi đã kết thúc'
          : 'Cuộc gọi thoại';

  return <Dialog.Root open={open} onOpenChange={(next) => { if (!next && status !== 'ended') onEnd(); }}>
    <Dialog.Portal>
      <Dialog.Overlay className="fixed inset-0 z-[70] bg-foreground/55 backdrop-blur-md"/>
      <Dialog.Content className="fixed left-1/2 top-1/2 z-[70] flex h-[min(84dvh,680px)] w-[min(94vw,520px)] -translate-x-1/2 -translate-y-1/2 flex-col items-center justify-between rounded-[2rem] border border-primary/15 bg-card px-6 py-10 shadow-2xl">
        <Dialog.Title className="sr-only">Cuộc gọi thoại</Dialog.Title>
        <Dialog.Description className="sr-only">{statusText}</Dialog.Description>
        <audio ref={audioRef} autoPlay playsInline/>
        {localStream && <audio autoPlay muted playsInline ref={(node) => { if (node) node.srcObject = localStream; }}/>}
        <div className="mt-12 flex flex-col items-center text-center">
          <span className={`mb-8 flex h-32 w-32 items-center justify-center rounded-full ${status === 'connected' ? 'bg-primary/10 ring-8 ring-primary/5' : 'bg-secondary'}`}>
            <Avatar name={callerName} className="h-28 w-28 border-4 border-card text-4xl"/>
          </span>
          <h2 className="max-w-full truncate text-2xl font-semibold">{callerName}</h2>
          <p className="mt-2 text-sm text-muted-foreground">{statusText}</p>
          {status === 'connecting' && <span className="mt-5 h-1 w-40 animate-pulse rounded-full bg-primary/50"/>}
          {error && <p role="alert" className="mt-5 max-w-sm text-sm text-destructive">{error}</p>}
        </div>
        <footer className="flex items-center justify-center gap-8">
          <Button variant={isMuted ? 'destructive' : 'secondary'} size="icon" className="h-14 w-14 rounded-full" onClick={onToggleMute} disabled={!localStream || status === 'ended'} aria-label={isMuted ? 'Bật microphone' : 'Tắt microphone'}>
            {isMuted ? <MicOff className="h-5 w-5"/> : <Mic className="h-5 w-5"/>}
          </Button>
          <Button variant="destructive" size="icon" className="h-16 w-16 rotate-[135deg] rounded-full" onClick={onEnd} aria-label="Kết thúc cuộc gọi" disabled={status === 'ended'}>
            {status === 'ended' ? <PhoneOff className="h-6 w-6"/> : <Phone className="h-6 w-6"/>}
          </Button>
        </footer>
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>;
}
