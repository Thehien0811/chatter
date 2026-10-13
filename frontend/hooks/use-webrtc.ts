'use client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { getSocket } from '@/lib/websocket-client';
export function useWebRTC(conversationId: string) {
  const peer = useRef<RTCPeerConnection | null>(null);
  const localStreamRef = useRef<MediaStream | null>(null);
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [remoteStream, setRemoteStream] = useState<MediaStream | null>(null);
  const [micEnabled, setMicEnabled] = useState(true);
  const [cameraEnabled, setCameraEnabled] = useState(true);
  const start = useCallback(async (video: boolean) => {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true, video });
    localStreamRef.current = stream;
    setLocalStream(stream);
    const connection = new RTCPeerConnection(); peer.current = connection;
    stream.getTracks().forEach((track) => connection.addTrack(track, stream));
    connection.ontrack = (event) => setRemoteStream(event.streams[0] ?? null);
    connection.onicecandidate = (event) => { if (event.candidate) getSocket()?.send(JSON.stringify({ type: 'call:ice', conversationId, candidate: event.candidate })); };
    return connection;
  }, [conversationId]);
  const toggleMic = () => { const enabled = !micEnabled; localStream?.getAudioTracks().forEach((track) => { track.enabled = enabled; }); setMicEnabled(enabled); };
  const toggleCamera = () => { const enabled = !cameraEnabled; localStream?.getVideoTracks().forEach((track) => { track.enabled = enabled; }); setCameraEnabled(enabled); };
  const end = useCallback(() => {
    peer.current?.close();
    peer.current = null;
    localStreamRef.current?.getTracks().forEach((track) => track.stop());
    localStreamRef.current = null;
    setLocalStream(null);
    setRemoteStream(null);
  }, []);
  useEffect(() => end, [end]);
  return { start, end, localStream, remoteStream, micEnabled, cameraEnabled, toggleMic, toggleCamera, peer };
}
