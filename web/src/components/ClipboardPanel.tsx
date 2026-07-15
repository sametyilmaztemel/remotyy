import { useEffect, useRef, useState, useCallback } from 'react';
import { useSignaling } from '../hooks/useSignaling';
import { useWebRTC } from '../hooks/useWebRTC';
import type { HostInfo, Message } from '../lib/protocol';

interface Props {
  host: HostInfo;
}

interface ClipboardEntry {
  text: string;
  timestamp: number;
  direction: 'from_host' | 'to_host';
}

export default function ClipboardPanel({ host }: Props) {
  const { client } = useSignaling();

  // ── Refs ──────────────────────────────────────
  const dcRef = useRef<RTCDataChannel | null>(null);
  const roomRef = useRef(`clipboard-${host.id}-${Date.now()}`);
  const mountedRef = useRef(true);
  const signalHandlersRef = useRef<Array<{ type: string; handler: (msg: Message) => void }>>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // ── State ─────────────────────────────────────
  const [active, setActive] = useState(false);
  const [status, setStatus] = useState<string>('disconnected');
  const [hostClipboard, setHostClipboard] = useState<string>('');
  const [localText, setLocalText] = useState<string>('');
  const [entries, setEntries] = useState<ClipboardEntry[]>([]);
  const [connectionStatus, setConnectionStatus] = useState<string>('disconnected');
  const [copied, setCopied] = useState(false);
  const [sent, setSent] = useState(false);

  // ── Cleanup signal handlers ───────────────────
  const cleanupSignalHandlers = useCallback(() => {
    if (!client) return;
    for (const { type, handler } of signalHandlersRef.current) {
      client.off(type as any, handler);
    }
    signalHandlersRef.current = [];
  }, [client]);

  // ── WebRTC ────────────────────────────────────
  const webrtc = useWebRTC({
    signal: client!,
    room: roomRef.current,
    onDataChannel(label, dc) {
      if (label === 'clipboard') {
        setupClipboardChannel(dc);
      }
    },
    onStateChange(state) {
      setConnectionStatus(state);
      if (state === 'connected' || state === 'completed') {
        setStatus('connected');
      } else if (state === 'failed' || state === 'disconnected' || state === 'closed') {
        setStatus('disconnected');
        setActive(false);
      }
    },
  });

  // ── Channel Setup ─────────────────────────────
  const setupClipboardChannel = useCallback((dc: RTCDataChannel) => {
    dcRef.current = dc;
    dc.binaryType = 'arraybuffer';

    dc.onmessage = (event: MessageEvent) => {
      if (!mountedRef.current) return;
      try {
        const raw = typeof event.data === 'string'
          ? event.data
          : new TextDecoder().decode(event.data as ArrayBuffer);
        const msg = JSON.parse(raw);

        // Host sends clipboard_data with clipboard_text
        if (msg.type === 'clipboard_data') {
          const text = msg.payload?.clipboard_text || msg.payload?.text || '';
          if (text) {
            setHostClipboard(text);
            setEntries(prev => [
              { text, timestamp: Date.now(), direction: 'from_host' },
              ...prev.slice(0, 49),
            ]);
          }
        }
      } catch (err) {
        console.error('[ClipboardPanel] Failed to parse message:', err);
      }
    };

    dc.onopen = () => {
      setStatus('connected');
      setConnectionStatus('connected');
    };

    dc.onclose = () => {
      if (mountedRef.current) {
        setStatus('disconnected');
        setConnectionStatus('disconnected');
      }
    };

    dc.onerror = (err) => {
      console.error('[ClipboardPanel] Data channel error:', err);
      setConnectionStatus('error');
    };
  }, []);

  // ── Wait for room_ready ───────────────────────
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

  // ── Authenticate ──────────────────────────────
  const authenticate = useCallback((): Promise<void> => {
    return new Promise((resolve, reject) => {
      const pc = webrtc.rtc.current;
      if (!pc) {
        reject(new Error('No peer connection'));
        return;
      }

      const authDc = pc.createDataChannel('auth');
      if (!authDc) {
        reject(new Error('Failed to create auth data channel'));
        return;
      }

      const authTimeout = setTimeout(() => {
        reject(new Error('Auth timeout'));
      }, 10000);

      authDc.onopen = () => {
        authDc.send(JSON.stringify({
          type: 'auth',
          payload: { password: '' },
        }));
      };

      authDc.onmessage = (event) => {
        try {
          const msg = JSON.parse(
            typeof event.data === 'string'
              ? event.data
              : new TextDecoder().decode(event.data as ArrayBuffer),
          );
          if (msg.type === 'auth_ok') {
            clearTimeout(authTimeout);
            resolve();
          } else if (msg.type === 'auth_fail') {
            clearTimeout(authTimeout);
            reject(new Error('Authentication failed'));
          }
        } catch (e) {
          // Ignore parse errors
        }
      };

      authDc.onerror = () => {
        clearTimeout(authTimeout);
        reject(new Error('Auth data channel error'));
      };
    });
  }, [webrtc.rtc]);

  // ── Start ─────────────────────────────────────
  const startClipboardSync = useCallback(async () => {
    try {
      setStatus('connecting');
      setActive(true);
      cleanupSignalHandlers();

      await webrtc.init(false);

      if (!mountedRef.current) return;

      client?.send({ type: 'connect', payload: { host_id: host.id } });

      const roomId = await waitForRoomReady();
      roomRef.current = roomId;

      if (!mountedRef.current) return;

      setStatus('connecting');
      await authenticate();

      if (!mountedRef.current) return;

      const pc = webrtc.rtc.current;
      if (!pc) throw new Error('No peer connection');

      const dc = pc.createDataChannel('clipboard');
      if (!dc) throw new Error('Failed to create clipboard data channel');
      dcRef.current = dc;

      await new Promise<void>((resolve, reject) => {
        const timeout = setTimeout(() => {
          reject(new Error('Clipboard data channel timeout'));
        }, 15000);

        dc.binaryType = 'arraybuffer';
        dc.onmessage = (event: MessageEvent) => {
          if (!mountedRef.current) return;
          try {
            const raw = typeof event.data === 'string'
              ? event.data
              : new TextDecoder().decode(event.data as ArrayBuffer);
            const msg = JSON.parse(raw);

            if (msg.type === 'clipboard_data') {
              const text = msg.payload?.clipboard_text || msg.payload?.text || '';
              if (text) {
                setHostClipboard(text);
                setEntries(prev => [
                  { text, timestamp: Date.now(), direction: 'from_host' },
                  ...prev.slice(0, 49),
                ]);
              }
            }
          } catch (err) {
            console.error('[ClipboardPanel] Parse error:', err);
          }
        };
        dc.onopen = () => {
          clearTimeout(timeout);
          setStatus('connected');
          setConnectionStatus('connected');
          resolve();
        };
        dc.onclose = () => {
          if (mountedRef.current) {
            setStatus('disconnected');
          }
        };
        dc.onerror = (err) => {
          clearTimeout(timeout);
          console.error('[ClipboardPanel] DC error:', err);
          reject(new Error('Clipboard data channel error'));
        };
      });
    } catch (err) {
      console.error('[ClipboardPanel] Failed to start:', err);
      setStatus('error');
      setActive(false);
      cleanupSignalHandlers();
      webrtc.close();
    }
  }, [webrtc, client, host.id, waitForRoomReady, authenticate, cleanupSignalHandlers]);

  // ── Stop ──────────────────────────────────────
  const stopClipboardSync = useCallback(() => {
    try {
      if (dcRef.current?.readyState === 'open') {
        // No specific stop message needed, just close
      }
    } catch (err) {
      console.error('[ClipboardPanel] Error stopping:', err);
    }

    cleanupSignalHandlers();
    webrtc.close();
    dcRef.current = null;

    if (mountedRef.current) {
      setActive(false);
      setStatus('disconnected');
      setConnectionStatus('disconnected');
    }
  }, [webrtc, cleanupSignalHandlers]);

  // ── Send over data channel ────────────────────
  const sendOverDC = useCallback((msg: unknown) => {
    try {
      if (dcRef.current?.readyState === 'open') {
        dcRef.current.send(JSON.stringify(msg));
      }
    } catch (err) {
      console.error('[ClipboardPanel] Send failed:', err);
    }
  }, []);

  // ── Read from Host ────────────────────────────
  const readFromHost = useCallback(async () => {
    // Send a clipboard_request to ask host for current clipboard
    sendOverDC({
      type: 'clipboard_request',
      payload: { request_id: `req-${Date.now()}` },
    });
  }, [sendOverDC]);

  // ── Copy host clipboard to browser clipboard ──
  const copyToBrowser = useCallback(async () => {
    if (!hostClipboard) return;
    try {
      await navigator.clipboard.writeText(hostClipboard);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('[ClipboardPanel] Failed to write to browser clipboard:', err);
    }
  }, [hostClipboard]);

  // ── Send to Host ──────────────────────────────
  const sendToHost = useCallback(() => {
    const text = localText || textareaRef.current?.value || '';
    if (!text.trim()) return;

    sendOverDC({
      type: 'clipboard_data',
      payload: {
        clipboard_text: text,
        timestamp: Date.now(),
      },
    });

    setEntries(prev => [
      { text, timestamp: Date.now(), direction: 'to_host' },
      ...prev.slice(0, 49),
    ]);
    setSent(true);
    setTimeout(() => setSent(false), 2000);
  }, [localText, sendOverDC]);

  // ── Read browser clipboard and populate textarea ──
  const pasteFromBrowser = useCallback(async () => {
    try {
      const text = await navigator.clipboard.readText();
      setLocalText(text);
    } catch (err) {
      console.error('[ClipboardPanel] Failed to read browser clipboard:', err);
    }
  }, []);

  // ── Cleanup on unmount ────────────────────────
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      cleanupSignalHandlers();
      dcRef.current = null;
      webrtc.close();
    };
  }, [webrtc, cleanupSignalHandlers]);

  // ── Format timestamp ──────────────────────────
  const formatTime = (ts: number) => {
    const d = new Date(ts);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  // ── Status label ──────────────────────────────
  const statusLabel = {
    disconnected: 'Disconnected',
    connecting: 'Connecting…',
    connected: 'Connected',
    error: 'Error',
  }[status] || status;

  // ── Render ────────────────────────────────────
  return (
    <div className="clipboard-panel">
      {/* ─── Header ──────────────────────────── */}
      <div className="clipboard-header">
        <div className="clipboard-title">
          <span className="clipboard-title-icon">📋</span>
          <span>{host.name} — Clipboard</span>
        </div>

        <div className="clipboard-header-center">
          <span className={`status-dot ${connectionStatus}`} />
          <span className="status-label">{statusLabel}</span>
        </div>

        <div className="clipboard-controls">
          {active && (
            <button className="btn-danger" onClick={stopClipboardSync}>
              ■ Disconnect
            </button>
          )}
          {!active && (
            <button className="btn-primary" onClick={startClipboardSync}>
              ▶ Connect Clipboard
            </button>
          )}
        </div>
      </div>

      {/* ─── Body ────────────────────────────── */}
      <div className="clipboard-body">
        {active && (
          <div className="clipboard-content">
            {/* ─── Read from Host ──────────────── */}
            <section className="clipboard-section">
              <h3>Host Clipboard</h3>
              <div className="clipboard-read-row">
                <button
                  className="clipboard-btn"
                  onClick={readFromHost}
                  disabled={connectionStatus !== 'connected'}
                  title="Request current host clipboard content"
                >
                  🔄 Read from Host
                </button>
              </div>

              {hostClipboard ? (
                <div className="clipboard-preview">
                  <div className="clipboard-preview-text">
                    <pre>{hostClipboard}</pre>
                  </div>
                  <button
                    className="clipboard-btn clipboard-btn-copy"
                    onClick={copyToBrowser}
                  >
                    {copied ? '✅ Copied!' : '📋 Copy to Browser'}
                  </button>
                </div>
              ) : (
                <div className="clipboard-empty">
                  <p>Click "Read from Host" to fetch clipboard contents</p>
                </div>
              )}
            </section>

            {/* ─── Divider ────────────────────── */}
            <div className="clipboard-divider">
              <span>⬌</span>
            </div>

            {/* ─── Send to Host ───────────────── */}
            <section className="clipboard-section">
              <h3>Send to Host</h3>
              <div className="clipboard-textarea-row">
                <textarea
                  ref={textareaRef}
                  className="clipboard-textarea"
                  placeholder="Type or paste text to send to host clipboard…"
                  value={localText}
                  onChange={(e) => setLocalText(e.target.value)}
                  rows={4}
                />
              </div>
              <div className="clipboard-send-row">
                <button
                  className="clipboard-btn clipboard-btn-secondary"
                  onClick={pasteFromBrowser}
                  title="Read from your browser clipboard"
                >
                  📥 Paste from Browser
                </button>
                <button
                  className="clipboard-btn clipboard-btn-primary"
                  onClick={sendToHost}
                  disabled={!localText.trim() || connectionStatus !== 'connected'}
                >
                  {sent ? '✅ Sent!' : '📤 Send to Host'}
                </button>
              </div>
            </section>

            {/* ─── History ────────────────────── */}
            {entries.length > 0 && (
              <section className="clipboard-section clipboard-history">
                <h3>History</h3>
                <div className="clipboard-history-list">
                  {entries.map((entry, i) => (
                    <div
                      key={`${entry.timestamp}-${i}`}
                      className={`clipboard-history-item ${entry.direction}`}
                    >
                      <span className="clipboard-history-direction">
                        {entry.direction === 'from_host' ? '⬇ Host' : '⬆ You'}
                      </span>
                      <span className="clipboard-history-time">
                        {formatTime(entry.timestamp)}
                      </span>
                      <span className="clipboard-history-text">
                        {entry.text.length > 80
                          ? entry.text.slice(0, 80) + '…'
                          : entry.text}
                      </span>
                    </div>
                  ))}
                </div>
              </section>
            )}
          </div>
        )}

        {!active && (
          <div className="clipboard-placeholder">
            <span className="placeholder-icon">📋</span>
            <p>Clipboard sharing inactive</p>
            <p className="hint">Click &lsquo;Connect Clipboard&rsquo; to sync clipboard with host</p>
          </div>
        )}
      </div>
    </div>
  );
}
