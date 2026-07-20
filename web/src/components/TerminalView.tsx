import { useEffect, useRef, useCallback, useState } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebLinksAddon } from 'xterm-addon-web-links';
import { useSignaling } from '../hooks/useSignaling';
import { useWebRTC } from '../hooks/useWebRTC';
import type { HostInfo, Message } from '../lib/protocol';
import type { ResizePayload } from '../lib/protocol';

const TERMINAL_THEME = {
  background: '#0f0f0f',
  foreground: '#c6c6c6',
  cursor: '#8b5cf6',
  cursorAccent: '#ffffff',
  selectionBackground: 'rgba(139, 92, 246, 0.25)',
  black: '#1a1a1a',
  red: '#e5484d',
  green: '#30a46c',
  yellow: '#f5a623',
  blue: '#6b8cff',
  magenta: '#c084fc',
  cyan: '#22d3ee',
  white: '#d4d4d4',
  brightBlack: '#404040',
  brightRed: '#ef4444',
  brightGreen: '#22c55e',
  brightYellow: '#facc15',
  brightBlue: '#60a5fa',
  brightMagenta: '#d8b4fe',
  brightCyan: '#67e8f9',
  brightWhite: '#ffffff',
};

interface Props {
  host: HostInfo;
  signalUrl: string;
}

type TermStatus = 'disconnected' | 'connecting' | 'connected' | 'error';

export default function TerminalView({ host }: Props) {
  const { client } = useSignaling();

  // Refs
  const termRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const dcRef = useRef<RTCDataChannel | null>(null);
  const roomRef = useRef(`term-${host.id}-${Date.now()}`);
  const mountedRef = useRef(true);
  const signalHandlersRef = useRef<Array<{ type: string; handler: (msg: Message) => void }>>([]);
  const [status, setStatus] = useState<TermStatus>('disconnected');
  const [statusMsg, setStatusMsg] = useState('');

  // Cleanup signal handlers
  const cleanupSignalHandlers = useCallback(() => {
    if (!client) return;
    for (const { type, handler } of signalHandlersRef.current) {
      client.off(type as any, handler);
    }
    signalHandlersRef.current = [];
  }, [client]);

  // WebRTC Hook
  const webrtc = useWebRTC({
    signal: client!,
    room: roomRef.current,
    onDataChannel(label, dc) {
      if (label !== 'terminal') return;
      dcRef.current = dc;
      dc.binaryType = 'arraybuffer';
      dc.onmessage = (event) => {
        if (!terminalRef.current) return;
        const data = typeof event.data === 'string'
          ? event.data
          : new TextDecoder().decode(event.data);
        terminalRef.current.write(data);
      };
      dc.onopen = () => setStatus('connected');
      dc.onclose = () => {
        if (mountedRef.current) setStatus('disconnected');
      };
      dc.onerror = () => {
        if (mountedRef.current) setStatus('error');
      };
    },
    onStateChange(state) {
      if (state === 'connected' || state === 'completed') {
        setStatus('connected');
      } else if (state === 'failed') {
        setStatus('error');
        setStatusMsg('WebRTC connection failed');
      } else if (state === 'disconnected' || state === 'closed') {
        setStatus('disconnected');
      }
    },
  });

  // Wait for room_ready from signaling
  const waitForRoomReady = useCallback((): Promise<string> => {
    return new Promise((resolve, reject) => {
      let cleanedUp = false;
      const cleanup = () => {
        if (cleanedUp) return;
        cleanedUp = true;
        clearTimeout(timeoutId);
        client?.off('room_ready', roomHandler);
        client?.off('error', errorHandler);
        signalHandlersRef.current = signalHandlersRef.current.filter(
          (h) => h.handler !== roomHandler && h.handler !== errorHandler,
        );
      };

      const roomHandler = (msg: Message) => {
        cleanup();
        const payload = msg.payload as { room?: string };
        if (payload?.room) {
          resolve(payload.room);
        } else {
          reject(new Error('room_ready missing room field'));
        }
      };
      const errorHandler = (msg: Message) => {
        cleanup();
        const payload = msg.payload as { message?: string };
        reject(new Error(payload?.message || 'Connection rejected'));
      };
      const timeoutId = setTimeout(() => {
        cleanup();
        reject(new Error('Timeout waiting for room_ready'));
      }, 15000);

      client?.on('room_ready', roomHandler);
      client?.on('error', errorHandler);
      signalHandlersRef.current.push(
        { type: 'room_ready', handler: roomHandler },
        { type: 'error', handler: errorHandler },
      );
    });
  }, [client]);

  // Main connection effect
  useEffect(() => {
    if (!termRef.current || !client || !host.id) return;
    let cancelled = false;

    (async () => {
      try {
        setStatus('connecting');
        setStatusMsg('Initializing WebRTC...');

        // Step 1: Init WebRTC as answerer FIRST (register signal listeners for offer/answer/ice)
        await webrtc.init(false);
        if (cancelled) return;

        // Step 2: Send connect to signaling to create a room with the host
        setStatusMsg('Connecting to host...');
        client.send({ type: 'connect', payload: { host_id: host.id } });

        // Step 3: Wait for room_ready
        const roomId = await waitForRoomReady();
        if (cancelled) return;
        roomRef.current = roomId;

        // Step 4: Wait for data channel to open (handled in onDataChannel)
        setStatusMsg('Establishing terminal channel...');
        await new Promise<void>((resolve, reject) => {
          const checkDc = setInterval(() => {
            if (dcRef.current?.readyState === 'open') {
              clearInterval(checkDc);
              clearTimeout(dcTimeout);
              resolve();
            }
          }, 100);
          const dcTimeout = setTimeout(() => {
            clearInterval(checkDc);
            reject(new Error('Terminal data channel timeout'));
          }, 20000);
        });

        if (cancelled) return;
        setStatus('connected');
        setStatusMsg('');
      } catch (err) {
        if (!cancelled) {
          console.error('[TerminalView] Connection failed:', err);
          setStatus('error');
          setStatusMsg(err instanceof Error ? err.message : 'Connection failed');
        }
      }
    })();

    return () => {
      cancelled = true;
      cleanupSignalHandlers();
      webrtc.close();
      dcRef.current = null;
    };
  }, []);

  // Terminal setup (separate effect, runs once)
  useEffect(() => {
    if (!termRef.current) return;

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: 'block',
      fontSize: 14,
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
      lineHeight: 1.35,
      letterSpacing: 0,
      theme: TERMINAL_THEME,
      allowTransparency: false,
      scrollback: 10000,
      smoothScrollDuration: 0,
      minimumContrastRatio: 4.5,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(new WebLinksAddon());

    term.open(termRef.current);
    fitAddon.fit();
    term.focus();

    terminalRef.current = term;
    fitRef.current = fitAddon;

    // Send terminal input via WebRTC data channel
    term.onData((data) => {
      if (dcRef.current?.readyState === 'open') {
        dcRef.current.send(data);
      }
    });

    // Send resize events
    const observer = new ResizeObserver(() => {
      try {
        fitAddon.fit();
        const dims = term.cols && term.rows ? { rows: term.rows, cols: term.cols } : null;
        if (dims && dcRef.current?.readyState === 'open') {
          const resizeMsg = JSON.stringify({ type: 'resize', payload: dims as ResizePayload });
          dcRef.current.send(resizeMsg);
        }
      } catch {
        // ignore
      }
    });
    observer.observe(termRef.current);

    return () => {
      observer.disconnect();
      term.dispose();
      terminalRef.current = null;
      fitRef.current = null;
    };
  }, []);

  // ── Connection splash overlay ──────────────────
  const showOverlay = status === 'connecting' || status === 'error';

  return (
    <div className="terminal-screen">
      {/* ── Header ── */}
      <div className="terminal-header">
        <div className="terminal-title">
          <span className={`term-indicator ${status}`} />
          <span className="term-hostname">{host.name}</span>
          <span className="term-features">
            {host.features?.includes('screen') ? '⎈' : ''}
          </span>
        </div>
        <div className="terminal-controls">
          <button className="btn-icon-sm" onClick={() => fitRef.current?.fit()} title="Fit terminal">
            ⊞
          </button>
          <button className="btn-icon-sm" onClick={() => terminalRef.current?.clear()} title="Clear">
            ✕
          </button>
        </div>
      </div>

      {/* ── Terminal container ── */}
      <div className="terminal-container" ref={termRef} />

      {/* ── Splash overlay ── */}
      {showOverlay && (
        <div className="term-overlay">
          {status === 'connecting' && (
            <div className="term-overlay-inner">
              <div className="term-spinner" />
              <div className="term-overlay-text">{statusMsg || 'Connecting...'}</div>
            </div>
          )}
          {status === 'error' && (
            <div className="term-overlay-inner">
              <div className="term-overlay-icon error">⚠</div>
              <div className="term-overlay-text error">{statusMsg || 'Connection error'}</div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
