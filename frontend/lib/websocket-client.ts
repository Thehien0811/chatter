import { getAccessToken } from '@/lib/api-client';

let socket: WebSocket | null = null;
let retryTimer: number | null = null;
let retryAttempt = 0;
let manuallyClosed = false;
const messageListeners = new Set<(event: MessageEvent<string>) => void>();

function createSocket(token: string) {
  const endpoint = process.env.NEXT_PUBLIC_WS_URL ?? 'ws://localhost:8080';
  const url = new URL(endpoint);
  url.pathname = '/api/v1/ws/presence';
  url.search = '';
  url.searchParams.set('token', token);

  const connection = new WebSocket(url.toString());
  socket = connection;
  connection.addEventListener('message', (event) => { for (const listener of messageListeners) listener(event as MessageEvent<string>); });
  connection.addEventListener('open', () => { retryAttempt = 0; });
  connection.addEventListener('close', () => {
    if (socket === connection) socket = null;
    if (manuallyClosed || retryAttempt >= 5 || typeof window === 'undefined') return;
    const delay = Math.min(1000 * 2 ** retryAttempt, 30_000);
    retryAttempt += 1;
    retryTimer = window.setTimeout(() => { retryTimer = null; getSocket(); }, delay);
  });
  return connection;
}

export function getSocket() {
  if (typeof window === 'undefined') return null;
  const token = getAccessToken();
  if (!token) return null;
  manuallyClosed = false;
  if (retryTimer !== null) { window.clearTimeout(retryTimer); retryTimer = null; }
  if (socket && (socket.readyState === WebSocket.CONNECTING || socket.readyState === WebSocket.OPEN)) return socket;
  return createSocket(token);
}

export function closeSocket() {
  manuallyClosed = true;
  retryAttempt = 0;
  if (retryTimer !== null && typeof window !== 'undefined') window.clearTimeout(retryTimer);
  retryTimer = null;
  const current = socket;
  socket = null;
  current?.close(1000, 'Signed out');
}

export function addSocketMessageListener(listener: (event: MessageEvent<string>) => void) {
  messageListeners.add(listener);
  return () => { messageListeners.delete(listener); };
}
