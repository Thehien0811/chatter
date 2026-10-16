'use client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { addSocketMessageListener, getSocket } from '@/lib/websocket-client';
import type { CallClientEvent, CallSignalEvent, CallType, SocketEvent } from '@/types/ws';

export type CallStatus = 'idle' | 'outgoing' | 'incoming' | 'connecting' | 'connected' | 'ended';
export type CallInfo = {
  id: string;
  conversationId: string;
  callType: CallType;
  peerId: string;
  direction: 'incoming' | 'outgoing';
};

const ringingTimeoutMs = 30_000;
const heartbeatIntervalMs = 12_000;
const endedNoticeMs = 4_000;

function iceServers(): RTCIceServer[] {
  const servers: RTCIceServer[] = [];
  const stunURL = process.env.NEXT_PUBLIC_STUN_URL;
  const turnURL = process.env.NEXT_PUBLIC_TURN_URL;
  const turnUser = process.env.NEXT_PUBLIC_TURN_USER;
  const turnPass = process.env.NEXT_PUBLIC_TURN_PASS;
  if (stunURL) servers.push({ urls: stunURL.split(',').map((url) => url.trim()).filter(Boolean) });
  if (turnURL && turnUser && turnPass) {
    servers.push({
      urls: turnURL.split(',').map((url) => url.trim()).filter(Boolean),
      username: turnUser,
      credential: turnPass,
    });
  }
  return servers;
}

export function useWebRTC() {
  const peer = useRef<RTCPeerConnection | null>(null);
  const localStreamRef = useRef<MediaStream | null>(null);
  const sessionRef = useRef<CallInfo | null>(null);
  const statusRef = useRef<CallStatus>('idle');
  const candidates = useRef<RTCIceCandidateInit[]>([]);
  const ringTimer = useRef<number | null>(null);
  const heartbeatTimer = useRef<number | null>(null);
  const restartTimer = useRef<number | null>(null);
  const endedTimer = useRef<number | null>(null);
  const restarting = useRef(false);
  const [status, setStatusValue] = useState<CallStatus>('idle');
  const [call, setCall] = useState<CallInfo | null>(null);
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [remoteStream, setRemoteStream] = useState<MediaStream | null>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [endReason, setEndReason] = useState('');
  const [error, setError] = useState('');

  const setStatus = useCallback((next: CallStatus) => {
    statusRef.current = next;
    setStatusValue(next);
  }, []);

  const sendSignal = useCallback((event: CallClientEvent) => {
    const socket = getSocket();
    if (!socket || socket.readyState !== WebSocket.OPEN) return false;
    socket.send(JSON.stringify({ ...event, ts: Date.now() }));
    return true;
  }, []);

  const clearTimers = useCallback(() => {
    if (ringTimer.current !== null) window.clearTimeout(ringTimer.current);
    if (heartbeatTimer.current !== null) window.clearInterval(heartbeatTimer.current);
    if (restartTimer.current !== null) window.clearTimeout(restartTimer.current);
    if (endedTimer.current !== null) window.clearTimeout(endedTimer.current);
    ringTimer.current = null;
    heartbeatTimer.current = null;
    restartTimer.current = null;
    endedTimer.current = null;
  }, []);

  const releaseMedia = useCallback(() => {
    peer.current?.close();
    peer.current = null;
    localStreamRef.current?.getTracks().forEach((track) => track.stop());
    localStreamRef.current = null;
    setLocalStream(null);
    setRemoteStream(null);
    candidates.current = [];
    restarting.current = false;
    setIsMuted(false);
  }, []);

  const finishCall = useCallback((reason: string, notifyPeer: boolean) => {
    const session = sessionRef.current;
    if (notifyPeer && session && !sendSignal({ type: 'call.end', payload: { callId: session.id, reason } })) {
      setError('Kết nối với máy chủ bị mất.');
    }
    clearTimers();
    setStatus('ended');
    releaseMedia();
    sessionRef.current = null;
    setEndReason(reason);
    endedTimer.current = window.setTimeout(() => {
      setCall(null);
      setEndReason('');
      setStatus('idle');
    }, endedNoticeMs);
  }, [clearTimers, releaseMedia, sendSignal, setStatus]);

  const startHeartbeat = useCallback((session: CallInfo) => {
    if (heartbeatTimer.current !== null) window.clearInterval(heartbeatTimer.current);
    heartbeatTimer.current = window.setInterval(() => {
      if (!sendSignal({ type: 'call.heartbeat', payload: { callId: session.id } })) {
        finishCall('network_error', false);
      }
    }, heartbeatIntervalMs);
  }, [finishCall, sendSignal]);

  const getAudioStream = useCallback(async () => {
    if (!navigator.mediaDevices?.getUserMedia) {
      throw new Error('Trình duyệt không hỗ trợ truy cập microphone.');
    }
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
    localStreamRef.current = stream;
    setLocalStream(stream);
    setIsMuted(false);
    return stream;
  }, []);

  const createPeer = useCallback((session: CallInfo) => {
    if (peer.current) return peer.current;
    const connection = new RTCPeerConnection({ iceServers: iceServers() });
    peer.current = connection;
    const stream = localStreamRef.current;
    stream?.getTracks().forEach((track) => connection.addTrack(track, stream));
    connection.ontrack = (event) => {
      const stream = event.streams[0] ?? new MediaStream([event.track]);
      setRemoteStream(stream);
    };
    connection.onicecandidate = (event) => {
      if (event.candidate) {
        sendSignal({ type: 'call.ice', payload: { callId: session.id, candidate: event.candidate.toJSON() } });
      }
    };
    connection.onconnectionstatechange = () => {
      if (connection.connectionState === 'connected') {
        if (ringTimer.current !== null) window.clearTimeout(ringTimer.current);
        if (restartTimer.current !== null) window.clearTimeout(restartTimer.current);
        ringTimer.current = null;
        restartTimer.current = null;
        setStatus('connected');
      } else if (connection.connectionState === 'failed') {
        if (!restarting.current) {
          restarting.current = true;
          restartTimer.current = window.setTimeout(() => finishCall('network_error', true), 10_000);
          void (async () => {
            try {
              connection.restartIce();
              const offer = await connection.createOffer({ iceRestart: true });
              await connection.setLocalDescription(offer);
              if (!sendSignal({ type: 'call.offer', payload: { callId: session.id, sdp: offer, iceRestart: true } })) {
                finishCall('network_error', false);
              }
            } catch {
              finishCall('network_error', true);
            }
          })();
        } else {
          finishCall('network_error', true);
        }
      } else if (connection.connectionState === 'closed' && statusRef.current !== 'ended') {
        finishCall('network_error', false);
      }
    };
    return connection;
  }, [finishCall, sendSignal, setStatus]);

  const flushCandidates = useCallback(async (connection: RTCPeerConnection) => {
    const queued = candidates.current;
    candidates.current = [];
    for (const candidate of queued) await connection.addIceCandidate(candidate);
  }, []);

  const beginRingTimeout = useCallback(() => {
    if (ringTimer.current !== null) window.clearTimeout(ringTimer.current);
    ringTimer.current = window.setTimeout(() => {
      if (statusRef.current === 'outgoing' || statusRef.current === 'incoming') {
        finishCall('timeout', true);
      }
    }, ringingTimeoutMs);
  }, [finishCall]);

  const startCall = useCallback(async (conversationId: string, type: CallType, peerId = '') => {
    if (statusRef.current !== 'idle') throw new Error('Đang có một cuộc gọi khác.');
    const callId = crypto.randomUUID();
    const session: CallInfo = { id: callId, conversationId, callType: type, peerId, direction: 'outgoing' };
    sessionRef.current = session;
    setCall(session);
    setError('');
    setEndReason('');
    setStatus('outgoing');
    if (!sendSignal({ type: 'call.invite', payload: { callId, conversationId, callType: type } })) {
      sessionRef.current = null;
      setCall(null);
      setStatus('idle');
      throw new Error('Không kết nối được máy chủ cuộc gọi.');
    }
    beginRingTimeout();
    startHeartbeat(session);
  }, [beginRingTimeout, sendSignal, setStatus, startHeartbeat]);

  const acceptCall = useCallback(async () => {
    const session = sessionRef.current;
    if (!session || session.direction !== 'incoming') return;
    try {
      await getAudioStream();
      createPeer(session);
      if (!sendSignal({ type: 'call.accept', payload: { callId: session.id } })) {
        throw new Error('Không kết nối được máy chủ cuộc gọi.');
      }
      if (ringTimer.current !== null) window.clearTimeout(ringTimer.current);
      ringTimer.current = null;
      setStatus('connecting');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Không thể truy cập microphone.');
      sendSignal({ type: 'call.reject', payload: { callId: session.id, reason: 'media_error' } });
      finishCall('network_error', false);
    }
  }, [createPeer, finishCall, getAudioStream, sendSignal, setStatus]);

  const rejectCall = useCallback(() => {
    const session = sessionRef.current;
    if (!session) return;
    sendSignal({ type: 'call.reject', payload: { callId: session.id, reason: 'declined' } });
    finishCall('declined', false);
  }, [finishCall, sendSignal]);

  const endCall = useCallback(() => finishCall('hangup', true), [finishCall]);

  const toggleMute = useCallback(() => {
    const enabled = isMuted;
    localStreamRef.current?.getAudioTracks().forEach((track) => { track.enabled = enabled; });
    setIsMuted(!enabled);
  }, [isMuted]);

  const handleSignal = useCallback(async (event: CallSignalEvent) => {
    if (event.type === 'call.invite') {
      if (statusRef.current !== 'idle') {
        sendSignal({ type: 'call.reject', payload: { callId: event.payload.callId, reason: 'busy' } });
        return;
      }
      const session: CallInfo = {
        id: event.payload.callId,
        conversationId: event.payload.conversationId,
        callType: event.payload.callType,
        peerId: event.payload.from,
        direction: 'incoming',
      };
      sessionRef.current = session;
      setCall(session);
      setError('');
      setStatus('incoming');
      beginRingTimeout();
      startHeartbeat(session);
      return;
    }
    const session = sessionRef.current;
    if (!session || event.payload.callId !== session.id) return;
    if (event.type === 'call.error') {
      setError(event.payload.error);
      finishCall('network_error', false);
      return;
    }
    try {
      if (event.type === 'call.accept' && session.direction === 'outgoing') {
        if (ringTimer.current !== null) window.clearTimeout(ringTimer.current);
        ringTimer.current = null;
        await getAudioStream();
        const connection = createPeer(session);
        const offer = await connection.createOffer();
        await connection.setLocalDescription(offer);
        if (!sendSignal({ type: 'call.offer', payload: { callId: session.id, sdp: offer } })) {
          throw new Error('Không thể gửi lời mời kết nối.');
        }
        setStatus('connecting');
      } else if (event.type === 'call.reject') {
        finishCall(event.payload.reason || 'declined', false);
      } else if (event.type === 'call.offer') {
        if (!localStreamRef.current) await getAudioStream();
        const connection = createPeer(session);
        await connection.setRemoteDescription(event.payload.sdp);
        await flushCandidates(connection);
        const answer = await connection.createAnswer();
        await connection.setLocalDescription(answer);
        if (!sendSignal({ type: 'call.answer', payload: { callId: session.id, sdp: answer } })) {
          throw new Error('Không thể gửi phản hồi kết nối.');
        }
        setStatus('connecting');
      } else if (event.type === 'call.answer') {
        const connection = peer.current;
        if (!connection) return;
        await connection.setRemoteDescription(event.payload.sdp);
        await flushCandidates(connection);
        setStatus('connecting');
      } else if (event.type === 'call.ice') {
        const connection = peer.current;
        if (connection?.remoteDescription) await connection.addIceCandidate(event.payload.candidate);
        else candidates.current.push(event.payload.candidate);
      } else if (event.type === 'call.end') {
        finishCall(event.payload.reason || 'hangup', false);
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Kết nối cuộc gọi thất bại.');
      finishCall('network_error', true);
    }
  }, [beginRingTimeout, createPeer, finishCall, flushCandidates, getAudioStream, sendSignal, setStatus, startHeartbeat]);

  useEffect(() => {
    const unsubscribe = addSocketMessageListener((message) => {
      try {
        const event = JSON.parse(message.data) as SocketEvent;
        if (event.type.startsWith('call.')) void handleSignal(event as CallSignalEvent);
      } catch {
        setError('Không thể đọc sự kiện cuộc gọi từ máy chủ.');
      }
    });
    return unsubscribe;
  }, [handleSignal]);

  useEffect(() => () => {
    clearTimers();
    peer.current?.close();
    localStreamRef.current?.getTracks().forEach((track) => track.stop());
  }, [clearTimers]);

  return {
    status,
    call,
    localStream,
    remoteStream,
    isMuted,
    error,
    endReason,
    startCall,
    acceptCall,
    rejectCall,
    endCall,
    toggleMute,
  };
}
