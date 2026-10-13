'use client';
import { FormEvent, KeyboardEvent, useCallback, useEffect, useRef, useState } from 'react';
import { ArrowLeft, Check, MessageCircle, Plus, Search, Send, UsersRound } from 'lucide-react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { Avatar } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { api, refreshAccessToken } from '@/lib/api-client';
import { addSocketMessageListener, getSocket } from '@/lib/websocket-client';
import { useFriendModalStore } from '@/hooks/use-friend-modal-store';
import { FriendModal } from '@/components/friend/friend-modal';

type ApiMessage = { id: string; conversation_id: string; sender_id: string; body: string; created_at: string };
type ApiConversation = { id: string; title: string; avatar_url?: string; participant: { id: string; username: string; display_name: string; presence?: string }; last_message?: ApiMessage; updated_at: string; unread_count: number };
type SocketEvent = { type: string; message?: ApiMessage; message_id?: string; error?: string };
type ApiUnreadMessage = { message_id: string; conversation_id: string; sender_id: string; body: string; created_at: string };
type UserPresence = { user_id: string; status: 'online' | 'offline' };
const clock = (value?: string) => value ? new Intl.DateTimeFormat('vi-VN', { hour: '2-digit', minute: '2-digit' }).format(new Date(value)) : '';

export function ChatWorkspace({ initialConversationId }: { initialConversationId?: string }) {
  const router = useRouter();
  const showFriendModal = useFriendModalStore((state) => state.show);
  const [conversations, setConversations] = useState<ApiConversation[]>([]);
  const [messages, setMessages] = useState<ApiMessage[]>([]);
  const [activeId, setActiveId] = useState<string | null>(initialConversationId ?? null);
  const [selfId, setSelfId] = useState('');
  const [search, setSearch] = useState('');
  const [draft, setDraft] = useState('');
  const [loadingConversations, setLoadingConversations] = useState(true);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [sending, setSending] = useState(false);
  const [connected, setConnected] = useState(false);
  const [participantPresence, setParticipantPresence] = useState<{ id: string; status: 'online' | 'offline' } | null>(null);
  const endRef = useRef<HTMLDivElement>(null);
  const historyRequest = useRef(0);
  const active = conversations.find((item) => item.id === activeId);

  // Conversation presence is only a snapshot; keep it fresh while the chat is open.
  useEffect(() => {
    const participantId = active?.participant?.id;
    if (!participantId) return;
    let live = true;
    const refreshPresence = () => {
      void api<UserPresence>(`/users/${encodeURIComponent(participantId)}/status`)
        .then((result) => { if (live) setParticipantPresence({ id: participantId, status: result.status === 'online' ? 'online' : 'offline' }); })
        .catch(() => { if (live) setParticipantPresence({ id: participantId, status: 'offline' }); });
    };
    refreshPresence();
    const timer = window.setInterval(refreshPresence, 5_000);
    return () => { live = false; window.clearInterval(timer); };
  }, [active?.participant?.id]);

  const loadConversations = useCallback(async () => {
    const response = await api<{ conversations: ApiConversation[] }>('/chat/conversations');
    return response.conversations ?? [];
  }, []);

  const loadMessages = useCallback(async (id: string) => {
    const response = await api<{ messages: ApiMessage[] }>(`/chat/conversations/${encodeURIComponent(id)}/messages?limit=50`);
    return (response.messages ?? []).sort((a, b) => a.created_at.localeCompare(b.created_at));
  }, []);

  useEffect(() => {
    let live = true;
    api<{ id: string }>('/profile').then((profile) => { if (live) setSelfId(profile.id); }).catch(() => undefined);
    loadConversations().then((items) => {
      if (!live) return;
      setConversations(items);
      if (initialConversationId) {
        setActiveId(initialConversationId);
        if (!items.some((item) => item.id === initialConversationId)) {
          setConversations((current) => current.some((item) => item.id === initialConversationId) ? current : [{ id: initialConversationId, title: `Cuộc trò chuyện ${initialConversationId.slice(0, 8)}`, participant: { id: '', username: '', display_name: '' }, updated_at: new Date().toISOString(), unread_count: 0 }, ...current]);
        }
      } else if (items[0]) setActiveId(items[0].id);
    }).catch((error: unknown) => toast.error(error instanceof Error ? error.message : 'Không tải được cuộc trò chuyện.')).finally(() => { if (live) setLoadingConversations(false); });
    // Restore an access token from the HttpOnly refresh-token cookie before opening the browser WebSocket.
    refreshAccessToken().then(() => { if (live) getSocket(); }).catch(() => undefined);
    return () => { live = false; };
  }, [initialConversationId, loadConversations]);

  useEffect(() => {
    if (!activeId) return;
    let live = true;
    const request = ++historyRequest.current;
    void Promise.resolve()
      .then(() => {
        if (live) setLoadingMessages(true);
        return loadMessages(activeId);
      })
      .then((history) => {
        if (live && request === historyRequest.current) setMessages(history);
      })
      .catch((error: unknown) => {
        if (live) toast.error(error instanceof Error ? error.message : 'Không tải được tin nhắn.');
      })
      .finally(() => {
        if (live && request === historyRequest.current) setLoadingMessages(false);
      });
    return () => {
      live = false;
      historyRequest.current += 1;
    };
  }, [activeId, loadMessages]);

  useEffect(() => {
    const unsubscribe = addSocketMessageListener((event) => {
      try {
        const data = JSON.parse(event.data) as SocketEvent;
        if (data.type === 'message' && data.message) {
          const incoming = data.message;
          void api(`/chat/messages/${encodeURIComponent(incoming.id)}/delivered`, { method: 'POST' }).catch(() => undefined);
          setConversations((items) => items.map((item) => item.id === incoming.conversation_id ? { ...item, last_message: incoming, updated_at: incoming.created_at, unread_count: item.id === activeId ? 0 : item.unread_count + 1 } : item).sort((a, b) => b.updated_at.localeCompare(a.updated_at)));
          setConversations((items) => items.some((item) => item.id === incoming.conversation_id) ? items : [{ id: incoming.conversation_id, title: `Cuộc trò chuyện ${incoming.conversation_id.slice(0, 8)}`, participant: { id: incoming.sender_id, username: incoming.sender_id, display_name: incoming.sender_id }, updated_at: incoming.created_at, unread_count: 1 }, ...items]);
          if (incoming.conversation_id === activeId) {
            setMessages((items) => items.some((item) => item.id === incoming.id) ? items : [...items.filter((message) => !(incoming.sender_id === selfId && message.id.startsWith('pending-'))), incoming]);
            if (incoming.sender_id === selfId) setSending(false);
          }
        } else if (data.type === 'message.accepted') {
          setSending(false);
          if (activeId && data.message_id) {
            setMessages((items) => items.map((item) => item.id.startsWith('pending-') ? { ...item, id: data.message_id! } : item));
          }
        } else if (data.type === 'error') {
          setSending(false);
          setMessages((items) => items.filter((item) => !item.id.startsWith('pending-')));
          toast.error(data.error ?? 'Không gửi được tin nhắn.');
        }
      } catch { /* Ignore unrelated or malformed socket frames. */ }
    });
    let attached: WebSocket | null = null;
    let recoveredSocket: WebSocket | null = null;
    const recoverUnread = async () => {
      try {
        const recovered: ApiMessage[] = [];
        // The endpoint caps each response at 100 and marks that page delivered,
        // so read again until the pending inbox has been drained.
        for (let page = 0; page < 20; page += 1) {
          const response = await api<{ messages: ApiUnreadMessage[] }>('/chat/unread?limit=100');
          const items = response.messages ?? [];
          recovered.push(...items.map((item) => ({
            id: item.message_id, conversation_id: item.conversation_id, sender_id: item.sender_id,
            body: item.body, created_at: item.created_at,
          })));
          if (items.length < 100) break;
        }
        const grouped = new Map<string, ApiMessage[]>();
        for (const message of recovered) grouped.set(message.conversation_id, [...(grouped.get(message.conversation_id) ?? []), message]);
        if (activeId && grouped.has(activeId)) {
          setMessages((current) => {
            const merged = new Map(current.map((item) => [item.id, item]));
            for (const item of grouped.get(activeId) ?? []) merged.set(item.id, item);
            return [...merged.values()].sort((a, b) => a.created_at.localeCompare(b.created_at));
          });
        }
        const items = await loadConversations();
        setConversations(items);
      } catch { /* Reconnect recovery will retry on the next WebSocket open. */ }
    };
    const update = () => {
      const current = getSocket();
      setConnected(current?.readyState === WebSocket.OPEN);
      if (current && current !== attached) {
        attached?.removeEventListener('open', update);
        attached?.removeEventListener('close', update);
        attached = current;
        current.addEventListener('open', update);
        current.addEventListener('close', update);
      }
      if (current?.readyState === WebSocket.OPEN && recoveredSocket !== current) {
        recoveredSocket = current;
        void recoverUnread();
      }
    };
    update();
    const timer = window.setInterval(update, 500);
    return () => { window.clearInterval(timer); unsubscribe(); attached?.removeEventListener('open', update); attached?.removeEventListener('close', update); };
  }, [activeId, loadConversations, loadMessages, selfId]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void loadConversations().then(setConversations).catch(() => undefined);
    }, 10_000);
    return () => window.clearInterval(timer);
  }, [loadConversations]);

  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages.length, activeId]);

  const selectConversation = (id: string) => {
    setActiveId(id);
    setConversations((items) => items.map((item) => item.id === id ? { ...item, unread_count: 0 } : item));
    router.push(`/chat/${encodeURIComponent(id)}`);
  };
  const createConversation = async (recipientId: string) => {
    try {
      const conversation = await api<ApiConversation>('/chat/conversations', {
        method: 'POST',
        body: JSON.stringify({ recipient_id: recipientId }),
      });
      setConversations((items) => [conversation, ...items.filter((item) => item.id !== conversation.id)]);
      setMessages([]);
      selectConversation(conversation.id);
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Không tạo được cuộc trò chuyện.'); }
  };
  const sendMessage = (event?: FormEvent) => {
    event?.preventDefault();
    const body = draft.trim();
    const socket = getSocket();
    if (!body || !activeId || !socket || socket.readyState !== WebSocket.OPEN || sending) {
      if (!socket || socket.readyState !== WebSocket.OPEN) toast.error('Đang kết nối lại máy chủ chat.');
      return;
    }
    const optimistic: ApiMessage = { id: `pending-${Date.now()}`, conversation_id: activeId, sender_id: selfId, body, created_at: new Date().toISOString() };
    socket.send(JSON.stringify({ type: 'send_message', conversation_id: activeId, body }));
    setMessages((items) => [...items, optimistic]);
    setConversations((items) => items.map((item) => item.id === activeId ? { ...item, last_message: optimistic, updated_at: optimistic.created_at, unread_count: 0 } : item));
    setSending(true);
    setDraft('');
  };
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); sendMessage(); } };
  const visible = conversations.filter((item) => `${item.title} ${item.participant?.username ?? ''} ${item.last_message?.body ?? ''}`.toLocaleLowerCase('vi').includes(search.toLocaleLowerCase('vi')));

  return <><div className="flex h-full min-h-0 w-full overflow-hidden bg-background">
    <aside className={`flex w-full shrink-0 flex-col border-r bg-background md:w-[340px] lg:w-[360px] ${active ? 'hidden md:flex' : 'flex'}`}>
      <header className="px-5 pb-4 pt-6 sm:px-6"><div className="flex items-start justify-between"><div><p className="text-sm font-medium text-primary">CHATTER</p><h1 className="mt-1 text-2xl font-bold tracking-tight">Tin nhắn</h1></div><div className="flex gap-2"><Button variant="outline" size="icon" className="rounded-full" aria-label="Bạn bè" title="Bạn bè" onClick={() => showFriendModal('add')}><UsersRound className="h-5 w-5"/></Button><Button variant="secondary" size="icon" className="rounded-full" aria-label="Tạo cuộc trò chuyện" onClick={() => showFriendModal('friends')}><Plus className="h-5 w-5"/></Button></div></div><div className="mt-4 flex items-center gap-2 rounded-xl border px-3 py-2"><span className={connected ? 'h-2 w-2 rounded-full bg-emerald-500' : 'h-2 w-2 rounded-full bg-amber-500'} /><p className="text-xs text-muted-foreground">{connected ? 'Đã kết nối' : 'Đang kết nối'}</p></div></header>
      <div className="px-4 pb-3 sm:px-5"><div className="relative"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"/><Input className="h-11 rounded-xl border-0 bg-muted/70 pl-9" placeholder="Tìm cuộc trò chuyện" value={search} onChange={(event) => setSearch(event.target.value)}/></div></div>
      <div className="flex-1 overflow-y-auto px-2 pb-3">{loadingConversations ? <p className="px-5 py-8 text-sm text-muted-foreground">Đang tải cuộc trò chuyện…</p> : visible.length ? visible.map((conversation) => <button key={conversation.id} onClick={() => selectConversation(conversation.id)} className={`flex w-full items-center gap-3 rounded-xl px-3 py-3 text-left ${activeId === conversation.id ? 'bg-primary/[0.08]' : 'hover:bg-muted/70'}`}><Avatar name={conversation.title || conversation.participant?.display_name || conversation.participant?.username || '?'} className="h-12 w-12"/><span className="min-w-0 flex-1"><span className="flex items-center justify-between gap-2"><span className="truncate text-sm font-semibold">{conversation.title || conversation.participant?.display_name || conversation.participant?.username}</span><time className="shrink-0 text-[10px] text-muted-foreground">{clock(conversation.updated_at)}</time></span><span className="mt-1 flex items-center justify-between gap-2"><span className="truncate text-xs text-muted-foreground">{conversation.last_message?.body ?? 'Chưa có tin nhắn'}</span>{conversation.unread_count > 0 && <span className="rounded-full bg-primary px-1.5 py-0.5 text-[10px] text-primary-foreground">{conversation.unread_count}</span>}</span></span></button>) : <div className="px-5 py-12 text-center"><MessageCircle className="mx-auto h-7 w-7 text-muted-foreground/70"/><p className="mt-3 text-sm text-muted-foreground">{search ? 'Không tìm thấy cuộc trò chuyện' : 'Chưa có cuộc trò chuyện nào'}</p></div>}</div>
    </aside>
    <section className={`min-w-0 flex-1 flex-col bg-muted/20 ${active ? 'flex' : 'hidden md:flex'}`}>
      {active ? <><header className="flex h-[76px] shrink-0 items-center gap-3 border-b bg-background px-3 sm:px-6"><Button variant="ghost" size="icon" className="md:hidden" aria-label="Quay lại danh sách" onClick={() => { setActiveId(null); router.push('/chat'); }}><ArrowLeft className="h-5 w-5"/></Button><Avatar name={active.title || active.participant?.display_name || active.participant?.username || '?'} className="h-11 w-11"/><div className="min-w-0 flex-1"><h2 className="truncate font-semibold">{active.title || active.participant?.display_name || active.participant?.username}</h2><p className="truncate text-xs text-muted-foreground">{participantPresence?.id === active.participant.id && participantPresence.status === 'online' ? 'Đang online' : 'Offline'}</p></div><span className="text-xs text-muted-foreground">{connected ? 'Trực tuyến' : 'Đang kết nối…'}</span></header>
        <div className="flex-1 overflow-y-auto px-4 sm:px-8"><div className="mx-auto flex min-h-full max-w-3xl flex-col justify-end py-6"><div className="space-y-1">{loadingMessages ? <p className="py-8 text-center text-sm text-muted-foreground">Đang tải tin nhắn…</p> : messages.map((message, index) => { const mine = message.sender_id === selfId; const previous = messages[index - 1]; return <div key={message.id} className={`flex ${mine ? 'justify-end' : 'justify-start'} ${previous?.sender_id === message.sender_id ? 'mt-1' : 'mt-4'}`}><div className={`max-w-[84%] rounded-2xl px-3.5 py-2.5 sm:max-w-[72%] ${mine ? 'rounded-br-md bg-primary text-primary-foreground' : 'rounded-bl-md border bg-background shadow-sm'}`}><p className="whitespace-pre-wrap break-words text-sm leading-5">{message.body}</p><div className={`mt-1 flex items-center justify-end gap-1 text-[10px] ${mine ? 'text-primary-foreground/70' : 'text-muted-foreground'}`}><time>{clock(message.created_at)}</time>{mine && !message.id.startsWith('pending-') && <Check className="h-3 w-3"/>}</div></div></div>; })}</div><div ref={endRef}/></div></div>
        <form className="shrink-0 border-t bg-background p-3 sm:px-6 sm:py-4" onSubmit={sendMessage}><div className="mx-auto flex max-w-3xl items-end gap-2 rounded-2xl border bg-muted/50 p-2"><textarea value={draft} onChange={(event) => setDraft(event.target.value)} onKeyDown={onKeyDown} rows={1} placeholder="Aa · nhập tin nhắn" className="max-h-32 min-h-10 flex-1 resize-y bg-transparent px-2 py-2.5 text-sm outline-none placeholder:text-muted-foreground"/><Button type="submit" size="icon" className="h-10 w-10 shrink-0 rounded-xl" aria-label="Gửi tin nhắn" disabled={!draft.trim() || sending || !connected}><Send className="h-4 w-4"/></Button></div></form>
      </> : <div className="m-auto hidden max-w-sm flex-col items-center px-6 text-center md:flex"><div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 text-primary"><MessageCircle className="h-7 w-7"/></div><h2 className="mt-5 text-lg font-semibold">Chọn một cuộc trò chuyện</h2><p className="mt-2 text-sm leading-6 text-muted-foreground">Chọn hội thoại ở danh sách để xem tin nhắn.</p></div>}
    </section>
  </div><FriendModal onSelectFriend={(friend) => { void createConversation(friend.id); }}/></>;
}
