'use client';
import { useCallback, useEffect, useState } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, MessageCircle, Search, UserPlus, X } from 'lucide-react';
import { toast } from 'sonner';
import { useFriendModalStore } from '@/hooks/use-friend-modal-store';
import { api } from '@/lib/api-client';
import { addSocketMessageListener } from '@/lib/websocket-client';
import { Avatar } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { PresenceIndicator } from '@/components/friend/presence-indicator';
import type { Friend } from '@/types/friend';

type Request = { id: string; from_user: Friend; to_user: Friend; created_at: string };

export function FriendModal({ onSelectFriend, socketConnected }: { onSelectFriend?: (friend: Friend) => void; socketConnected: boolean }) {
  const { open, tab, close, setTab } = useFriendModalStore();
  const queryClient = useQueryClient();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<Friend[]>([]);
  const [incoming, setIncoming] = useState<Request[]>([]);
  const [outgoing, setOutgoing] = useState<Request[]>([]);
  const [loading, setLoading] = useState(false);
  const [busyId, setBusyId] = useState('');

  const loadFriends = useCallback(async () => {
    const response = await api<{ friends: Friend[] }>('/friends');
    const latest = response.friends ?? [];
    const cached = queryClient.getQueryData<Friend[]>(['friends']) ?? [];
    return latest.map((friend) => {
      const previous = cached.find((item) => item.id === friend.id);
      return previous && (previous.presence_version ?? -1) > (friend.presence_version ?? -1)
        ? { ...friend, presence: previous.presence, presence_version: previous.presence_version }
        : friend;
    });
  }, [queryClient]);
  const friendsQuery = useQuery({
    queryKey: ['friends'],
    queryFn: loadFriends,
    enabled: open,
    staleTime: 15_000,
    refetchInterval: open ? 30_000 : false,
  });
  const friends = friendsQuery.data ?? [];
  const loadRequests = async () => {
    const [inResponse, outResponse] = await Promise.all([
      api<{ requests: Request[] }>('/friend-requests?direction=incoming'),
      api<{ requests: Request[] }>('/friend-requests?direction=outgoing'),
    ]);
    setIncoming(inResponse.requests ?? []);
    setOutgoing(outResponse.requests ?? []);
  };

  useEffect(() => {
    if (!open) return;
    let live = true;
    void Promise.resolve()
      .then(() => {
        if (!live) return;
        setLoading(true);
        return loadRequests();
      })
      .catch((error: unknown) => {
        if (live) toast.error(error instanceof Error ? error.message : 'Không tải được dữ liệu bạn bè.');
      })
      .finally(() => { if (live) setLoading(false); });
    return () => { live = false; };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    return addSocketMessageListener((event) => {
      try {
        const data = JSON.parse(event.data) as { type?: string };
        if (data.type !== 'friend.request' && data.type !== 'friend.accepted') return;
        void Promise.all([
          queryClient.invalidateQueries({ queryKey: ['friends'] }),
          loadRequests(),
        ]).catch((error: unknown) => {
          toast.error(error instanceof Error ? error.message : 'Không cập nhật được danh sách bạn bè.');
        });
        if (data.type === 'friend.request') toast.info('Bạn nhận được lời mời kết bạn mới.');
        if (data.type === 'friend.accepted') toast.success('Lời mời kết bạn đã được chấp nhận.');
      } catch { /* Ignore unrelated or malformed socket frames. */ }
    });
  }, [open, queryClient]);

  useEffect(() => {
    if (!open || tab !== 'add') return;
    const value = query.trim();
    if (value.length < 2) return;
    let live = true;
    const timer = window.setTimeout(() => {
      api<{ users: Friend[] }>(`/users/search?q=${encodeURIComponent(value)}`)
        .then((response) => { if (live) setResults(response.users ?? []); })
        .catch((error: unknown) => { if (live) toast.error(error instanceof Error ? error.message : 'Không tìm được người dùng.'); });
    }, 250);
    return () => { live = false; window.clearTimeout(timer); };
  }, [open, query, tab]);

  const sendRequest = async (user: Friend) => {
    setBusyId(user.id);
    try {
      await api('/friend-requests', { method: 'POST', body: JSON.stringify({ recipient_id: user.id }) });
      await loadRequests();
      toast.success(`Đã gửi lời mời kết bạn tới ${user.display_name || user.username}.`);
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Không gửi được lời mời.'); }
    finally { setBusyId(''); }
  };
  const respond = async (request: Request, action: 'accept' | 'reject') => {
    setBusyId(request.id);
    try {
      await api(`/friend-requests/${encodeURIComponent(request.id)}/${action}`, { method: 'POST' });
      await Promise.all([queryClient.invalidateQueries({ queryKey: ['friends'] }), loadRequests()]);
      if (action === 'accept') toast.success('Đã chấp nhận lời mời kết bạn.');
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Không cập nhật được lời mời.'); }
    finally { setBusyId(''); }
  };
  const isFriend = (id: string) => friends.some((friend) => friend.id === id);
  const isPending = (id: string) => outgoing.some((request) => request.to_user.id === id);

  return <Dialog.Root open={open} onOpenChange={(next) => { if (!next) close(); }}><Dialog.Portal>
    <Dialog.Overlay className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"/>
    <Dialog.Content className="fixed left-1/2 top-1/2 z-50 flex max-h-[85dvh] w-[min(94vw,500px)] -translate-x-1/2 -translate-y-1/2 flex-col rounded-3xl border border-primary/10 bg-card p-5 shadow-2xl sm:p-6">
      <div className="flex items-start justify-between"><div><Dialog.Title className="text-xl font-semibold">Bạn bè</Dialog.Title><Dialog.Description className="mt-1 text-sm text-muted-foreground">Kết nối với mọi người trên Chatter.</Dialog.Description></div><Dialog.Close asChild><Button variant="ghost" size="icon" aria-label="Đóng"><X className="h-4 w-4"/></Button></Dialog.Close></div>
      <div className="mt-5 grid grid-cols-3 rounded-xl bg-muted p-1 text-sm">
        {([['add', 'Tìm bạn'], ['received', `Lời mời${incoming.length ? ` (${incoming.length})` : ''}`], ['friends', `Bạn bè (${friends.length})`]] as const).map(([key, label]) => <button key={key} onClick={() => setTab(key)} className={`rounded-lg px-2 py-2 ${tab === key ? 'bg-background font-medium shadow-sm' : 'text-muted-foreground'}`}>{label}</button>)}
      </div>
      {tab === 'add' ? <div className="mt-4 flex min-h-0 flex-1 flex-col">
        <div className="relative"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"/><Input autoFocus value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Tìm theo tên hoặc username" className="pl-9"/></div>
        <div className="mt-3 min-h-0 flex-1 overflow-y-auto">
          {query.trim().length < 2 ? <p className="py-6 text-center text-sm text-muted-foreground">Nhập ít nhất 2 ký tự để tìm người dùng.</p> : results.length ? results.map((user) => <div key={user.id} className="flex items-center gap-3 border-b py-3 last:border-0"><Avatar name={user.display_name || user.username} className="h-10 w-10"/><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{user.display_name || user.username}</p><p className="truncate text-xs text-muted-foreground">@{user.username}</p></div>{isFriend(user.id) ? <span className="text-xs text-muted-foreground">Bạn bè</span> : isPending(user.id) ? <span className="text-xs text-muted-foreground">Đã gửi</span> : <Button size="sm" variant="outline" disabled={busyId === user.id} onClick={() => void sendRequest(user)}><UserPlus className="mr-1.5 h-4 w-4"/>Kết bạn</Button>}</div>) : <p className="py-6 text-center text-sm text-muted-foreground">Chưa có kết quả.</p>}
        </div>
        {outgoing.length > 0 && <p className="mt-2 text-xs text-muted-foreground">Đã gửi {outgoing.length} lời mời đang chờ phản hồi.</p>}
      </div> : tab === 'received' ? <div className="mt-4 min-h-0 flex-1 overflow-y-auto">
        {loading ? <p className="py-8 text-center text-sm text-muted-foreground">Đang tải lời mời…</p> : incoming.length ? incoming.map((request) => <div key={request.id} className="flex items-center gap-3 border-b py-3 last:border-0"><Avatar name={request.from_user.display_name || request.from_user.username} className="h-10 w-10"/><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{request.from_user.display_name || request.from_user.username}</p><p className="truncate text-xs text-muted-foreground">@{request.from_user.username}</p></div><Button size="sm" disabled={busyId === request.id} onClick={() => void respond(request, 'accept')}><Check className="mr-1.5 h-4 w-4"/>Chấp nhận</Button><Button size="sm" variant="ghost" disabled={busyId === request.id} onClick={() => void respond(request, 'reject')}>Từ chối</Button></div>) : <p className="py-8 text-center text-sm text-muted-foreground">Bạn chưa có lời mời kết bạn nào.</p>}
      </div> : <div className="mt-4 min-h-0 flex-1 overflow-y-auto">
        {friendsQuery.isPending ? <div className="space-y-3 py-3">{Array.from({ length: 4 }, (_, index) => <Skeleton key={index} className="h-14 rounded-xl"/>)}</div> : friends.length ? friends.map((friend) => {
          const presence = socketConnected ? friend.presence ?? 'unknown' : 'unknown';
          return <div key={friend.id} className="flex items-center gap-3 rounded-2xl px-2 py-3 transition-colors hover:bg-secondary/70">
            <span className="relative"><Avatar name={friend.display_name || friend.username} className="h-11 w-11"/><i aria-hidden="true" className={`absolute bottom-0 right-0 h-3.5 w-3.5 rounded-full border-2 border-card ${presence === 'offline' ? 'bg-presence-offline' : 'bg-presence-online'}`}/></span>
            <div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold">{friend.display_name || friend.username}</p><p className="truncate text-xs text-muted-foreground">@{friend.username}</p><PresenceIndicator status={presence} className="mt-1"/></div>
            {onSelectFriend && <Button size="sm" variant="secondary" className="rounded-full" onClick={() => { close(); onSelectFriend(friend); }}><MessageCircle className="mr-1.5 h-4 w-4"/>Nhắn tin</Button>}
          </div>;
        }) : <p className="py-8 text-center text-sm text-muted-foreground">Chưa có bạn bè. Tìm và gửi lời mời kết bạn.</p>}
      </div>}
    </Dialog.Content>
  </Dialog.Portal></Dialog.Root>;
}
