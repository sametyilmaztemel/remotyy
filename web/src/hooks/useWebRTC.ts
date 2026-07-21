import { useState, useRef, useCallback } from 'react';
import { WebRTCClient } from '../lib/webrtc';
import type { SignalingClient } from '../lib/signaling';
import type { Message } from '../lib/protocol';

interface UseWebRTCOptions {
  signal: SignalingClient;
  room: string;
  onDataChannel?: (label: string, dc: RTCDataChannel) => void;
  onStateChange?: (state: string) => void;
}

export function useWebRTC({ signal, room, onDataChannel, onStateChange }: UseWebRTCOptions) {
  const [connected, setConnected] = useState(false);
  const [state, setState] = useState('new');
  const rtcRef = useRef<WebRTCClient | null>(null);
  const signalHandlers = useRef<Array<{ type: string; handler: (msg: Message) => void }>>([]);

  const init = useCallback(async (isOfferer: boolean) => {
    // Clean up any previous listeners before registering new ones
    for (const { type, handler } of signalHandlers.current) {
      signal.off(type as any, handler);
    }
    signalHandlers.current = [];

    const rtc = new WebRTCClient();
    rtcRef.current = rtc;

    rtc.onIceCandidateCb((candidate) => {
      signal.send({ type: 'ice_candidate', payload: candidate, room });
    });

    rtc.onDataChannelCb((label, dc) => {
      onDataChannel?.(label, dc);
    });

    rtc.onStateChangeCb((state) => {
      setState(state);
      if (state === 'connected' || state === 'completed') {
        setConnected(true);
      }
      onStateChange?.(state);
    });

    // Listen for signaling messages
    const handleSignal = (msg: Message) => {
      switch (msg.type) {
        case 'offer':
          if (!isOfferer) {
            rtc.handleOffer(msg.payload as RTCSessionDescriptionInit).then(answer => {
              signal.send({ type: 'answer', payload: answer, room });
            });
          }
          break;
        case 'answer':
          if (isOfferer) {
            rtc.handleAnswer(msg.payload as RTCSessionDescriptionInit);
          }
          break;
        case 'ice_candidate':
          rtc.addIceCandidate(msg.payload as RTCIceCandidateInit);
          break;
      }
    };

    signal.on('offer', handleSignal);
    signal.on('answer', handleSignal);
    signal.on('ice_candidate', handleSignal);
    signalHandlers.current = [
      { type: 'offer', handler: handleSignal },
      { type: 'answer', handler: handleSignal },
      { type: 'ice_candidate', handler: handleSignal },
    ];

    if (isOfferer) {
      const offer = await rtc.createOffer();
      signal.send({ type: 'offer', payload: offer, room });
    }

    return rtc;
  }, [signal, room, onDataChannel, onStateChange]);

  const close = useCallback(() => {
    // Remove signal listeners to prevent leaks
    for (const { type, handler } of signalHandlers.current) {
      signal.off(type as any, handler);
    }
    signalHandlers.current = [];

    rtcRef.current?.close();
    rtcRef.current = null;
    setConnected(false);
    setState('closed');
  }, [signal]);

  const send = useCallback((label: string, data: ArrayBuffer | string) => {
    // Find the data channel by label
    const dc = (rtcRef.current as any)?.dataChannels?.get?.(label);
    if (dc?.readyState === 'open') {
      dc.send(data);
    }
  }, []);

  return { init, close, send, connected, state, rtc: rtcRef };
}
