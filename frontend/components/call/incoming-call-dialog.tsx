'use client';
import { useEffect } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import { Phone, PhoneOff } from 'lucide-react';
import { toast } from 'sonner';
import { Avatar } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';

export function IncomingCallDialog({
  open,
  callerName,
  video,
  onAccept,
  onDecline,
}: {
  open: boolean;
  callerName: string;
  video: boolean;
  onAccept: () => void;
  onDecline: () => void;
}) {
  useEffect(() => {
    if (!open) return;
    const context = new AudioContext();
    let timer: number | null = null;
    let cancelled = false;
    const ring = () => {
      const oscillator = context.createOscillator();
      const gain = context.createGain();
      oscillator.frequency.value = 520;
      gain.gain.setValueAtTime(0.0001, context.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.12, context.currentTime + 0.03);
      gain.gain.exponentialRampToValueAtTime(0.0001, context.currentTime + 0.65);
      oscillator.connect(gain);
      gain.connect(context.destination);
      oscillator.start();
      oscillator.stop(context.currentTime + 0.7);
    };
    void context.resume().then(() => {
      if (cancelled) return;
      ring();
      timer = window.setInterval(ring, 1500);
    }).catch(() => toast.error('Không thể phát âm thanh chuông cuộc gọi.'));
    return () => {
      cancelled = true;
      if (timer !== null) window.clearInterval(timer);
      void context.close();
    };
  }, [open]);

  return <Dialog.Root open={open} onOpenChange={(next) => { if (!next) onDecline(); }}>
    <Dialog.Portal>
      <Dialog.Overlay className="fixed inset-0 z-[80] bg-foreground/50 backdrop-blur-sm"/>
      <Dialog.Content className="fixed left-1/2 top-1/2 z-[80] w-[min(92vw,400px)] -translate-x-1/2 -translate-y-1/2 rounded-3xl border bg-card p-7 text-center shadow-2xl">
        <Dialog.Title className="sr-only">Cuộc gọi đến</Dialog.Title>
        <Dialog.Description className="text-sm text-muted-foreground">
          Cuộc gọi {video ? 'video' : 'thoại'} đến
        </Dialog.Description>
        <div className="mx-auto mt-7 flex h-24 w-24 animate-pulse items-center justify-center rounded-full bg-primary/10">
          <Avatar name={callerName} className="h-20 w-20 border-4 border-card text-2xl"/>
        </div>
        <p className="mt-5 truncate text-xl font-semibold">{callerName}</p>
        <p className="mt-1 text-sm text-muted-foreground">Đang gọi cho bạn…</p>
        <div className="mt-8 flex items-center justify-center gap-8">
          <Button variant="destructive" size="icon" className="h-14 w-14 rounded-full" onClick={onDecline} aria-label="Từ chối cuộc gọi">
            <PhoneOff className="h-5 w-5"/>
          </Button>
          <Button size="icon" className="h-14 w-14 rounded-full bg-emerald-600 text-white hover:bg-emerald-700" onClick={onAccept} aria-label="Trả lời cuộc gọi">
            <Phone className="h-5 w-5"/>
          </Button>
        </div>
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>;
}
