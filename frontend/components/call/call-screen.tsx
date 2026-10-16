'use client';
import { useEffect, useRef, useState } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import { Camera, CameraOff, Mic, MicOff, Video, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useMediaPreview } from '@/hooks/use-media-preview';
function LocalVideo({ stream }: { stream: MediaStream | null }) {
  const ref = useRef<HTMLVideoElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.srcObject = stream;
  }, [stream]);
  return <video ref={ref} autoPlay playsInline muted className="h-full w-full object-cover"/>;
}

function mediaErrorMessage(reason: unknown) {
  if (reason instanceof DOMException) {
    if (reason.name === 'NotAllowedError' || reason.name === 'PermissionDeniedError') {
      return 'Hãy cấp quyền camera và microphone cho trình duyệt rồi thử lại.';
    }
    if (reason.name === 'NotFoundError' || reason.name === 'DevicesNotFoundError') {
      return 'Không tìm thấy camera hoặc microphone trên thiết bị này.';
    }
    if (reason.name === 'NotReadableError' || reason.name === 'TrackStartError') {
      return 'Camera hoặc microphone đang được một ứng dụng khác sử dụng.';
    }
  }
  return reason instanceof Error ? reason.message : 'Không thể truy cập camera hoặc microphone.';
}

export function VideoPreviewDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
 const call = useMediaPreview();
 const { startPreview, end } = call;
 const [error, setError] = useState('');
 useEffect(() => {
   if (!open) return;
   let active = true;
   startPreview().then(() => {
     if (active) setError('');
   }).catch((reason: unknown) => {
     if (!active) return;
     setError(mediaErrorMessage(reason));
   });
   return () => {
     active = false;
     end();
   };
 }, [open, startPreview, end]);

 const close = () => {
   end();
   setError('');
   onClose();
 };

 return <Dialog.Root open={open} onOpenChange={(next) => { if (!next) close(); }}>
   <Dialog.Portal>
     <Dialog.Overlay className="fixed inset-0 z-[70] bg-foreground/40 backdrop-blur-sm"/>
     <Dialog.Content className="fixed left-1/2 top-1/2 z-[70] w-[min(94vw,720px)] -translate-x-1/2 -translate-y-1/2 overflow-hidden rounded-3xl border border-primary/15 bg-card shadow-2xl">
       <header className="flex items-center justify-between border-b border-primary/10 px-5 py-4">
         <div><Dialog.Title className="font-semibold">Xem trước cuộc gọi video</Dialog.Title><Dialog.Description className="mt-1 text-xs text-muted-foreground">Đây chỉ là preview cục bộ, chưa kết nối với người nhận.</Dialog.Description></div>
         <Button variant="ghost" size="icon" onClick={close} aria-label="Đóng preview"><X className="h-4 w-4"/></Button>
       </header>
       <div className="relative aspect-video bg-secondary">
         {call.localStream ? <LocalVideo stream={call.localStream}/> : <div className="flex h-full flex-col items-center justify-center gap-3 px-5 text-center"><CameraOff className="h-9 w-9 text-muted-foreground"/><p className="max-w-md text-sm text-muted-foreground">{error || 'Đang yêu cầu quyền sử dụng camera và microphone…'}</p>{error && <Button variant="outline" className="rounded-full" onClick={() => { setError(''); void call.startPreview().catch((reason: unknown) => setError(mediaErrorMessage(reason))); }}>Thử lại</Button>}</div>}
       </div>
       <footer className="flex items-center justify-center gap-3 border-t border-primary/10 px-5 py-4">
         <Button variant={call.micEnabled ? 'secondary' : 'destructive'} size="icon" className="rounded-full" onClick={call.toggleMic} aria-label={call.micEnabled ? 'Tắt microphone' : 'Bật microphone'}><span className="relative"><Mic className="h-4 w-4"/>{!call.micEnabled && <MicOff className="absolute -right-1 -top-1 h-3 w-3"/>}</span></Button>
         <Button variant={call.cameraEnabled ? 'secondary' : 'destructive'} size="icon" className="rounded-full" onClick={call.toggleCamera} aria-label={call.cameraEnabled ? 'Tắt camera' : 'Bật camera'}>{call.cameraEnabled ? <Video className="h-4 w-4"/> : <Camera className="h-4 w-4"/>}</Button>
         <Button variant="destructive" className="rounded-full px-5" onClick={close}>Đóng preview</Button>
       </footer>
     </Dialog.Content>
   </Dialog.Portal>
 </Dialog.Root>;
}
