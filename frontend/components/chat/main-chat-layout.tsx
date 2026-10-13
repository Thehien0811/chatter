'use client';
import * as Dialog from '@radix-ui/react-dialog';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { LogOut, MessageCircle, Server, ShieldCheck, X } from 'lucide-react';
import { useRouter } from 'next/navigation';
import { useEffect, useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import { api, getAccessToken, refreshAccessToken, setAccessToken } from '@/lib/api-client';
import { closeSocket, getSocket } from '@/lib/websocket-client';
import { Avatar } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { ProfileEditor } from '@/components/profile/profile-editor';
import type { User } from '@/types/user';

type Health = { status: string };
type Presence = { user_id: string; status: 'online' | 'offline' };

export function MainChatLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [accountOpen, setAccountOpen] = useState(false);
  const queryClient = useQueryClient();
  const profile = useQuery({ queryKey: ['profile'], queryFn: () => api<User>('/profile'), retry: false });
  const profileId = profile.data?.id;
  const presence = useQuery({ queryKey: ['presence', profileId], queryFn: () => api<Presence>(`/users/${encodeURIComponent(profileId!)}/status`), enabled: Boolean(profileId), staleTime: 3_000, refetchInterval: 10_000 });
  const health = useQuery({ queryKey: ['api-health'], queryFn: () => api<Health>('/health'), retry: false, staleTime: 60_000 });
  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    let activeSocket: WebSocket | null = null;
    const refreshStatus = () => { void queryClient.invalidateQueries({ queryKey: ['presence', profileId] }); };
    const connect = async () => {
      try {
        if (!getAccessToken()) {
          await refreshAccessToken();
        }
        if (cancelled) return;
        activeSocket = getSocket();
        if (!activeSocket) return;
        if (activeSocket.readyState === WebSocket.OPEN) refreshStatus();
        else activeSocket.addEventListener('open', refreshStatus);
        activeSocket.addEventListener('close', refreshStatus);
      } catch {
        // The profile remains usable when the presence socket is unavailable.
      }
    };
    void connect();
    return () => {
      cancelled = true;
      activeSocket?.removeEventListener('open', refreshStatus);
      activeSocket?.removeEventListener('close', refreshStatus);
    };
  }, [profileId, queryClient]);
  const logout = async () => {
    closeSocket();
    setAccessToken(null);
    try {
      await api('/auth/logout', { method: 'POST' });
      router.replace('/login');
      router.refresh();
    } catch {
      toast.error('Không thể đăng xuất.');
    }
  };

  return <Dialog.Root open={accountOpen} onOpenChange={setAccountOpen}>
    <main className="flex h-dvh flex-col overflow-hidden bg-background md:flex-row">
      <aside className="z-20 flex h-14 w-full shrink-0 items-center justify-between border-b bg-background px-3 md:h-dvh md:w-[72px] md:flex-col md:justify-start md:border-b-0 md:border-r md:px-0 md:py-5">
        <div className="flex items-center gap-2 md:flex-col md:gap-0"><div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary text-lg font-bold text-primary-foreground md:mb-8">c</div><span className="font-semibold md:hidden">Chatter</span></div>
        <Button variant="secondary" size="icon" className="rounded-xl md:rounded-2xl" aria-label="Tin nhắn"><MessageCircle className="h-5 w-5"/></Button>
        <div className="flex items-center gap-1 md:mt-auto md:flex-col md:gap-3">
          <Dialog.Trigger asChild><button className="rounded-full outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring" aria-label="Tài khoản"><span className="relative block"><Avatar name={profile.data?.display_name || profile.data?.username || 'U'} className="h-9 w-9 cursor-pointer border-2 border-background"/>{presence.data?.status === 'online' && <i className="absolute bottom-0 right-0 h-3 w-3 rounded-full border-2 border-background bg-emerald-500"/>}</span></button></Dialog.Trigger>
          <Button variant="ghost" size="icon" className="rounded-xl md:rounded-2xl" aria-label="Đăng xuất" title="Đăng xuất" onClick={() => void logout()}><LogOut className="h-5 w-5"/></Button>
        </div>
      </aside>
      <div className="min-h-0 min-w-0 flex-1">{children}</div>
    </main>
    <Dialog.Portal><Dialog.Overlay className="fixed inset-0 z-50 bg-black/45 backdrop-blur-sm"/><Dialog.Content className="fixed left-1/2 top-1/2 z-50 max-h-[90dvh] w-[min(94vw,440px)] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-2xl border bg-background p-5 shadow-2xl sm:p-6">
      <div className="flex items-start justify-between"><div><Dialog.Title className="text-lg font-semibold">Tài khoản</Dialog.Title><Dialog.Description className="mt-1 text-sm text-muted-foreground">Thông tin hồ sơ từ Chatter API.</Dialog.Description></div><Dialog.Close asChild><Button variant="ghost" size="icon" aria-label="Đóng"><X className="h-4 w-4"/></Button></Dialog.Close></div>
      {profile.data ? <><div className="mt-5 flex items-center gap-3 rounded-xl bg-muted/60 p-3"><Avatar name={profile.data.display_name || profile.data.username} className="h-12 w-12"/><div className="min-w-0"><p className="truncate font-semibold">{profile.data.display_name || profile.data.username}</p><p className="truncate text-sm text-muted-foreground">@{profile.data.username}</p></div></div><dl className="mt-4 space-y-3 text-sm"><div><dt className="text-xs text-muted-foreground">Email</dt><dd className="mt-0.5 break-all">{profile.data.email || 'Chưa cập nhật'}</dd></div><div><dt className="text-xs text-muted-foreground">Trạng thái</dt><dd className="mt-0.5 flex items-center gap-2"><i className={`h-2 w-2 rounded-full ${presence.data?.status === 'online' ? 'bg-emerald-500' : 'bg-muted-foreground/50'}`}/>{presence.data?.status === 'online' ? 'Đang online' : presence.data?.status === 'offline' ? 'Offline' : 'Đang kiểm tra…'}</dd></div><div><dt className="text-xs text-muted-foreground">Tin nhắn trạng thái</dt><dd className="mt-0.5">{profile.data.status_message || 'Chưa đặt trạng thái'}</dd></div></dl><ProfileEditor profile={profile.data}/></> : <p className="mt-5 rounded-xl bg-muted p-4 text-sm text-muted-foreground">{profile.isPending ? 'Đang tải hồ sơ…' : 'Không tải được hồ sơ. Phiên đăng nhập có thể đã hết hạn.'}</p>}
      <div className="mt-5 rounded-xl border p-3"><div className="flex items-center gap-2 text-sm"><Server className="h-4 w-4 text-muted-foreground"/><span>API Gateway</span><span className={`ml-auto h-2 w-2 rounded-full ${health.data?.status === 'ok' ? 'bg-emerald-500' : health.isPending ? 'bg-amber-500' : 'bg-destructive'}`}/><span className="text-xs text-muted-foreground">{health.data?.status === 'ok' ? 'Hoạt động' : health.isPending ? 'Đang kiểm tra' : 'Không kết nối'}</span></div><div className="mt-2 flex items-start gap-2 text-xs leading-5 text-muted-foreground"><ShieldCheck className="mt-0.5 h-4 w-4 shrink-0"/><span>Kết nối WebSocket cập nhật trạng thái online. Tin nhắn được đồng bộ qua WebSocket.</span></div></div>
      <Button variant="outline" className="mt-4 w-full justify-start gap-2" onClick={() => void logout()}><LogOut className="h-4 w-4"/>Đăng xuất</Button>
    </Dialog.Content></Dialog.Portal>
  </Dialog.Root>;
}
