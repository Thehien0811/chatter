import { getAccessToken } from '@/lib/api-client';

let socket: WebSocket | null = null;
let retryTimer: number | null = null;
let retryAttempt = 0;
let manuallyClosed = false;
let onlineListenerRegistered = false;
const messageListeners = new Set<(event: MessageEvent<string>) => void>();
const socketStateListeners = new Set<(connection: WebSocket | null) => void>();

function notifySocketState(connection: WebSocket | null) {
  for (const listener of socketStateListeners) listener(connection);
}

function scheduleReconnect(delay: number) {
  if (manuallyClosed || typeof window === 'undefined' || retryTimer !== null) return;
  retryTimer = window.setTimeout(() => {
    retryTimer = null;
    getSocket();
  }, delay);
}

function handleNetworkOnline() {
  if (manuallyClosed || socket?.readyState === WebSocket.OPEN || socket?.readyState === WebSocket.CONNECTING) return;
  if (retryTimer !== null) window.clearTimeout(retryTimer);
  retryTimer = null;
  retryAttempt = 0;
  scheduleReconnect(0);
}

function createSocket(token: string) {
  const endpoint = process.env.NEXT_PUBLIC_WS_URL ?? 'ws://localhost:8080';
  const url = new URL(endpoint);
  url.pathname = '/api/v1/ws/presence';
  url.search = '';
  url.searchParams.set('token', token);

  const connection = new WebSocket(url.toString());
  socket = connection;
  notifySocketState(connection);
  if (!onlineListenerRegistered) {
    window.addEventListener('online', handleNetworkOnline);
    onlineListenerRegistered = true;
  }
  connection.addEventListener('message', (event) => { for (const listener of messageListeners) listener(event as MessageEvent<string>); });
  connection.addEventListener('open', () => {
    retryAttempt = 0;
    notifySocketState(connection);
  });
  connection.addEventListener('close', () => {
    if (socket === connection) {
      socket = null;
      notifySocketState(null);
    }
    if (manuallyClosed || typeof window === 'undefined') return;
    const delay = Math.min(1000 * 2 ** retryAttempt, 30_000);
    retryAttempt = Math.min(retryAttempt + 1, 5);
    scheduleReconnect(delay);
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
  if (onlineListenerRegistered && typeof window !== 'undefined') {
    window.removeEventListener('online', handleNetworkOnline);
    onlineListenerRegistered = false;
  }
  const current = socket;
  socket = null;
  notifySocketState(null);
  current?.close(1000, 'Signed out');
}

export function addSocketMessageListener(listener: (event: MessageEvent<string>) => void) {
  messageListeners.add(listener);
  return () => { messageListeners.delete(listener); };
}

export function addSocketStateListener(listener: (connection: WebSocket | null) => void) {
  socketStateListeners.add(listener);
  listener(socket);
  return () => { socketStateListeners.delete(listener); };
}
