'use client';
import { useCallback, useEffect, useRef, useState } from 'react';

export function useMediaPreview() {
  const streamRef = useRef<MediaStream | null>(null);
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [micEnabled, setMicEnabled] = useState(true);
  const [cameraEnabled, setCameraEnabled] = useState(true);

  const startPreview = useCallback(async () => {
    if (!navigator.mediaDevices?.getUserMedia) {
      throw new Error('Trình duyệt không hỗ trợ truy cập camera và microphone.');
    }
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: true });
    streamRef.current = stream;
    setLocalStream(stream);
    setMicEnabled(true);
    setCameraEnabled(true);
    return stream;
  }, []);

  const end = useCallback(() => {
    streamRef.current?.getTracks().forEach((track) => track.stop());
    streamRef.current = null;
    setLocalStream(null);
    setMicEnabled(true);
    setCameraEnabled(true);
  }, []);

  const toggleMic = useCallback(() => {
    const enabled = !micEnabled;
    streamRef.current?.getAudioTracks().forEach((track) => { track.enabled = enabled; });
    setMicEnabled(enabled);
  }, [micEnabled]);

  const toggleCamera = useCallback(() => {
    const enabled = !cameraEnabled;
    streamRef.current?.getVideoTracks().forEach((track) => { track.enabled = enabled; });
    setCameraEnabled(enabled);
  }, [cameraEnabled]);

  useEffect(() => end, [end]);

  return { localStream, micEnabled, cameraEnabled, startPreview, end, toggleMic, toggleCamera };
}
