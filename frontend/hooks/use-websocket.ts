'use client';
import { useEffect } from 'react';
import { getSocket } from '@/lib/websocket-client';
export function useWebSocket(onMessage?: (event: MessageEvent<string>) => void) {
  useEffect(() => {
    const socket = getSocket();
    if (!socket || !onMessage) return;
    socket.addEventListener('message', onMessage);
    return () => socket.removeEventListener('message', onMessage);
  }, [onMessage]);
  return { send: (payload: unknown) => { const socket = getSocket(); if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(payload)); } };
}
