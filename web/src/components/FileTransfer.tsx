import { useState, useRef, useCallback, useEffect } from 'react';
import { useSignaling } from '../hooks/useSignaling';
import { useWebRTC } from '../hooks/useWebRTC';
import type { HostInfo, Message, FileChunkPayload, FileRequestPayload } from '../lib/protocol';

interface Props {
  host: HostInfo;
  signalUrl: string;
}

interface Transfer {
  id: string;
  name: string;
  size: number;
  chunkSize: number;
  totalChunks: number;
  sentChunks: number;
  bytesSent: number;
  progress: number;
  status: 'pending' | 'active' | 'complete' | 'error' | 'cancelled';
  error?: string;
}

const CHUNK_SIZE = 65536; // 64 KB — matches Go's DefaultChunkSize

// ── Helpers ─────────────────────────────────────────

function arrayBufferToBase64(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let binary = '';
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

async function sha256Hex(data: ArrayBuffer): Promise<string> {
  const hash = await crypto.subtle.digest('SHA-256', data);
  const view = new Uint8Array(hash);
  return Array.from(view)
    .map(b => b.toString(16).padStart(2, '0'))
    .join('');
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec < 1024) return `${bytesPerSec.toFixed(0)} B/s`;
  if (bytesPerSec < 1024 * 1024) return `${(bytesPerSec / 1024).toFixed(1)} KB/s`;
  return `${(bytesPerSec / (1024 * 1024)).toFixed(1)} MB/s`;
}

// ── Component ───────────────────────────────────────

export default function FileTransfer({ host }: Props) {
  const { client } = useSignaling();

  // State
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [dragging, setDragging] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState('disconnected');
  const [error, setError] = useState('');

  // Refs
  const fileInput = useRef<HTMLInputElement>(null);
  const dcRef = useRef<RTCDataChannel | null>(null);       // file data channel
  const authDcRef = useRef<RTCDataChannel | null>(null);    // auth data channel
  const roomRef = useRef(`file-${host.id}-${Date.now()}`);
  const mountedRef = useRef(true);
  const signalHandlersRef = useRef<Array<{ type: string; handler: (msg: Message) => void }>>([]);

  // ── WebRTC Hook ──────────────────────────────────

  const webrtc = useWebRTC({
    signal: client!,
    room: roomRef.current,
    onDataChannel(label, dc) {
      if (label === 'file') {
        dcRef.current = dc;
        dc.binaryType = 'arraybuffer';
        dc.onmessage = (event) => {
          try {
            const raw = typeof event.data === 'string'
              ? event.data
              : new TextDecoder().decode(event.data as ArrayBuffer);
            const msg = JSON.parse(raw);
            handleFileMessage(msg);
          } catch (e) {
            console.error('[FileTransfer] Failed to parse DC message:', e);
          }
        };
        dc.onopen = () => setConnectionStatus('connected');
        dc.onclose = () => { if (mountedRef.current) setConnectionStatus('disconnected'); };
        dc.onerror = () => setConnectionStatus('error');
      }
    },
    onStateChange(state) {
      if (state === 'connected' || state === 'completed') {
        setConnectionStatus('connected');
      } else if (state === 'failed') {
        setConnectionStatus('error');
      } else if (state === 'disconnected' || state === 'closed') {
        if (mountedRef.current) setConnectionStatus('disconnected');
      }
    },
  });

  // ── Signal Handler Cleanup ────────────────────────

  const cleanupSignalHandlers = useCallback(() => {
    if (!client) return;
    for (const { type, handler } of signalHandlersRef.current) {
      client.off(type as any, handler);
    }
    signalHandlersRef.current = [];
  }, [client]);

  // ── Incoming File Protocol Messages ──────────────

  const handleFileMessage = useCallback((msg: any) => {
    if (!mountedRef.current) return;

    switch (msg.type) {
      case 'file_accept':
        setTransfers(prev =>
          prev.map(t =>
            t.id === msg.payload?.transfer_id ? { ...t, status: 'active' as const } : t,
          ),
        );
        break;

      case 'file_reject':
        setTransfers(prev =>
          prev.map(t =>
            t.id === msg.payload?.transfer_id
              ? { ...t, status: 'error' as const, error: msg.payload?.reason || 'Rejected by host' }
              : t,
          ),
        );
        break;

      case 'file_progress':
        // Optional: update progress based on server feedback
        break;

      case 'file_complete':
        setTransfers(prev =>
          prev.map(t =>
            t.id === msg.payload?.transfer_id
              ? { ...t, status: 'complete' as const, progress: 100 }
              : t,
          ),
        );
        break;

      case 'file_cancel':
        setTransfers(prev =>
          prev.map(t =>
            t.id === msg.payload?.transfer_id
              ? { ...t, status: 'cancelled' as const }
              : t,
          ),
        );
        break;
    }
  }, []);

  // ── Wait for room_ready ───────────────────────────

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
          h => h.handler !== roomHandler && h.handler !== errorHandler,
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

  // ── Authenticate via auth data channel ───────────

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
      authDcRef.current = authDc;

      const authTimeout = setTimeout(() => reject(new Error('Auth timeout')), 10000);

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
        } catch (_) {
          // ignore parse errors on auth channel
        }
      };

      authDc.onerror = () => {
        clearTimeout(authTimeout);
        reject(new Error('Auth data channel error'));
      };
    });
  }, [webrtc.rtc]);

  // ── Connect to Host ────────────────────────────────

  const connect = useCallback(async () => {
    if (!client) return;
    try {
      setConnectionStatus('connecting');
      setError('');
      cleanupSignalHandlers();

      // Step 1: init WebRTC as non-offerer (host creates the offer)
      await webrtc.init(false);

      if (!mountedRef.current) return;

      // Step 2: send connect to signal server to open a room
      client.send({ type: 'connect', payload: { host_id: host.id } });

      // Step 3: wait for room_ready
      const roomId = await waitForRoomReady();
      roomRef.current = roomId;

      if (!mountedRef.current) return;

      // Step 4: authenticate via auth data channel
      await authenticate();

      if (!mountedRef.current) return;

      // Step 5: create the file data channel
      const pc = webrtc.rtc.current;
      if (!pc) throw new Error('No peer connection for file channel');

      const dc = pc.createDataChannel('file');
      if (!dc) throw new Error('Failed to create file data channel');
      dcRef.current = dc;
      dc.binaryType = 'arraybuffer';

      // Step 6: wait for data channel to open
      await new Promise<void>((resolve, reject) => {
        const dcTimeout = setTimeout(() => reject(new Error('File data channel timeout')), 15000);

        dc.onmessage = (event) => {
          try {
            const raw = typeof event.data === 'string'
              ? event.data
              : new TextDecoder().decode(event.data as ArrayBuffer);
            const msg = JSON.parse(raw);
            handleFileMessage(msg);
          } catch (e) {
            console.error('[FileTransfer] Failed to parse DC message:', e);
          }
        };

        dc.onopen = () => {
          clearTimeout(dcTimeout);
          setConnectionStatus('connected');
          resolve();
        };

        dc.onclose = () => {
          if (mountedRef.current) setConnectionStatus('disconnected');
        };

        dc.onerror = () => {
          clearTimeout(dcTimeout);
          reject(new Error('File data channel error'));
        };
      });
    } catch (err: any) {
      console.error('[FileTransfer] Connection failed:', err);
      setConnectionStatus('error');
      setError(err.message || 'Connection failed');
      cleanupSignalHandlers();
      webrtc.close();
    }
  }, [webrtc, client, host.id, waitForRoomReady, authenticate, cleanupSignalHandlers, handleFileMessage]);

  // ── Disconnect ────────────────────────────────────

  const disconnect = useCallback(() => {
    // Send file_cancel for any active transfers
    for (const t of transfers) {
      if (t.status === 'active' || t.status === 'pending') {
        sendOnDC(JSON.stringify({ type: 'file_cancel', payload: { transfer_id: t.id } }));
      }
    }
    cleanupSignalHandlers();
    webrtc.close();
    dcRef.current = null;
    authDcRef.current = null;
    if (mountedRef.current) {
      setConnectionStatus('disconnected');
    }
  }, [webrtc, cleanupSignalHandlers, transfers]);

  // ── Auto-connect on mount ─────────────────────────

  useEffect(() => {
    mountedRef.current = true;
    connect();
    return () => {
      mountedRef.current = false;
      disconnect();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ── Send helper ───────────────────────────────────

  const sendOnDC = (data: string | ArrayBuffer) => {
    try {
      if (dcRef.current?.readyState === 'open') {
        // RTCDataChannel.send accepts string, Blob, ArrayBuffer, or ArrayBufferView
        (dcRef.current as any).send(data);
      }
    } catch (err) {
      console.error('[FileTransfer] Send failed:', err);
    }
  };

  // ── Send file_request message ─────────────────────

  const sendFileRequest = (transferId: string, file: File) => {
    const req: FileRequestPayload = {
      transfer_id: transferId,
      name: file.name,
      size: file.size,
      mime_type: file.type || 'application/octet-stream',
      chunk_size: CHUNK_SIZE,
    };
    sendOnDC(JSON.stringify({ type: 'file_request', payload: req }));
  };

  // ── Start a file transfer ─────────────────────────

  const startFileTransfer = async (file: File) => {
    const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
    const transferId = `tf-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

    const transfer: Transfer = {
      id: transferId,
      name: file.name,
      size: file.size,
      chunkSize: CHUNK_SIZE,
      totalChunks,
      sentChunks: 0,
      bytesSent: 0,
      progress: 0,
      status: 'pending',
    };

    setTransfers(prev => [...prev, transfer]);

    try {
      // Send file_request immediately (host may ignore if handleFileChannel is a stub)
      sendFileRequest(transferId, file);
      setTransfers(prev =>
        prev.map(t => (t.id === transferId ? { ...t, status: 'active' as const } : t)),
      );

      const startTime = Date.now();
      let bytesSent = 0;

      for (let i = 0; i < totalChunks; i++) {
        if (!mountedRef.current) break;
        if (dcRef.current?.readyState !== 'open') {
          throw new Error('Data channel closed during transfer');
        }

        const start = i * CHUNK_SIZE;
        const end = Math.min(start + CHUNK_SIZE, file.size);
        const chunkBlob = file.slice(start, end);
        const chunkBuffer = await chunkBlob.arrayBuffer();
        const checksum = await sha256Hex(chunkBuffer);
        const base64Data = arrayBufferToBase64(chunkBuffer);

        const chunkPayload: FileChunkPayload = {
          transfer_id: transferId,
          index: i,
          data: base64Data,
          checksum,
        };

        sendOnDC(JSON.stringify({ type: 'file_chunk', payload: chunkPayload }));

        bytesSent += chunkBuffer.byteLength;
        const elapsed = (Date.now() - startTime) / 1000;
        const speed = elapsed > 0 ? bytesSent / elapsed : 0;

        setTransfers(prev =>
          prev.map(t =>
            t.id === transferId
              ? {
                  ...t,
                  sentChunks: i + 1,
                  bytesSent,
                  progress: (bytesSent / file.size) * 100,
                  status: 'active' as const,
                }
              : t,
          ),
        );

        // Yield to main thread periodically to keep UI responsive
        if (i % 8 === 0 && i > 0) {
          await new Promise(r => setTimeout(r, 0));
        }
      }

      if (!mountedRef.current) return;

      // Send file_complete
      sendOnDC(JSON.stringify({ type: 'file_complete', payload: { transfer_id: transferId } }));
      setTransfers(prev =>
        prev.map(t =>
          t.id === transferId ? { ...t, status: 'complete' as const, progress: 100 } : t,
        ),
      );
    } catch (err: any) {
      if (mountedRef.current) {
        setTransfers(prev =>
          prev.map(t =>
            t.id === transferId
              ? { ...t, status: 'error' as const, error: err.message || 'Transfer failed' }
              : t,
          ),
        );
      }
    }
  };

  // ── Drag / File select handlers ──────────────────

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragging(false);
    const files = Array.from(e.dataTransfer.files);
    files.forEach(startFileTransfer);
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    files.forEach(startFileTransfer);
    // Reset so the same file can be selected again
    e.target.value = '';
  };

  // ── Render ─────────────────────────────────────────

  return (
    <div className="file-transfer">
      {/* Header */}
      <div className="file-header">
        <h2>
          <span className={`status-dot ${connectionStatus}`} />
          {host.name} &mdash; File Transfer
        </h2>
        <div className="file-header-right">
          <span className="connection-label">
            {connectionStatus === 'connected' && '✔ Connected'}
            {connectionStatus === 'connecting' && '⟳ Connecting...'}
            {connectionStatus === 'error' && '✖ Error'}
            {connectionStatus === 'disconnected' && '— Disconnected'}
          </span>
          {(connectionStatus === 'connected' || connectionStatus === 'error') && (
            <button className="btn-small" onClick={disconnect}>
              {connectionStatus === 'connected' ? 'Disconnect' : 'Reconnect'}
            </button>
          )}
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="file-error">
          <span>⚠</span> {error}
        </div>
      )}

      {/* Drop zone */}
      <div
        className={`drop-zone ${dragging ? 'dragging' : ''} ${connectionStatus !== 'connected' ? 'disabled' : ''}`}
        onDragOver={e => { e.preventDefault(); if (connectionStatus === 'connected') setDragging(true); }}
        onDragLeave={() => setDragging(false)}
        onDrop={handleDrop}
        onClick={() => connectionStatus === 'connected' && fileInput.current?.click()}
      >
        <input
          ref={fileInput}
          type="file"
          multiple
          onChange={handleFileSelect}
          style={{ display: 'none' }}
        />
        <span className="drop-icon">
          {connectionStatus === 'connected' ? '📁' : '🔒'}
        </span>
        <p>
          {connectionStatus === 'connected'
            ? 'Drop files here or click to select'
            : 'Connecting to host...'}
        </p>
        {connectionStatus === 'connected' && (
          <p className="drop-hint">Files are chunked (64 KB) and sent via WebRTC data channel</p>
        )}
      </div>

      {/* Transfer list */}
      {transfers.length > 0 && (
        <div className="transfer-list">
          <h3>Transfers</h3>
          {transfers.map(t => (
            <div key={t.id} className="transfer-item">
              <div className="transfer-info">
                <span className="transfer-name" title={t.name}>{t.name}</span>
                <span className="transfer-size">{formatSize(t.size)}</span>
              </div>

              <div className="transfer-progress">
                <div
                  className={`progress-bar ${t.status}`}
                  style={{ width: `${t.progress}%` }}
                />
              </div>

              <div className="transfer-status">
                {t.status === 'pending' && <span>⟳ Queued...</span>}
                {t.status === 'active' && (
                  <span>
                    {formatSpeed(
                      t.bytesSent / (Math.max(Date.now() - (/* rough start */ Date.now() - 1000), 1) / 1000),
                    )}
                    {' — '}
                    {t.sentChunks}/{t.totalChunks} chunks
                    {' — '}
                    {t.progress.toFixed(1)}%
                  </span>
                )}
                {t.status === 'complete' && <span>✅ Complete</span>}
                {t.status === 'error' && <span>❌ {t.error || 'Error'}</span>}
                {t.status === 'cancelled' && <span>🚫 Cancelled</span>}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Empty state */}
      {transfers.length === 0 && connectionStatus === 'connected' && (
        <div className="transfer-empty">
          <p>No active transfers. Drop a file above to start.</p>
        </div>
      )}
    </div>
  );
}
