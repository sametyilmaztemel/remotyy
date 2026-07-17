import { useEffect, useState, useRef, useCallback } from 'react';
import { useSignaling } from '../hooks/useSignaling';
import type { HostInfo, Message } from '../lib/protocol';
import QRCode from './QRCode';

interface Props {
  onSelect: (host: HostInfo) => void;
}

export default function HostList({ onSelect }: Props) {
  const { client, status, connect } = useSignaling();
  const [hosts, setHosts] = useState<HostInfo[]>([]);
  const [error, setError] = useState('');
  const [qrHost, setQrHost] = useState<HostInfo | null>(null);
  const skipConnectRef = useRef(false);

  useEffect(() => {
    if (!client) return;

    client.on('host_list', (msg: Message) => {
      const payload = msg.payload as { hosts: HostInfo[] };
      if (payload?.hosts) setHosts(payload.hosts);
    });
    client.on('error', (msg: Message) => {
      const payload = msg.payload as { message?: string };
      setError(payload?.message || 'Error');
    });

    connect().then(() => {
      client.send({ type: 'list_hosts' });
    }).catch(err => setError(err.message));
  }, [client, connect]);

  const handleConnect = useCallback((host: HostInfo) => {
    // HostList only calls onSelect — the actual WebRTC connection
    // is initiated by TerminalView or ScreenViewer when they mount.
    // Do NOT send 'connect' here — that would create a room prematurely
    // and leak a room_ready handler that redirects to terminal on every
    // subsequent room_ready event (breaking screen share, file transfer, etc.)
    skipConnectRef.current = true;
    onSelect(host);
  }, [onSelect]);

  return (
    <div className="host-list-screen">
      <div className="host-list-header">
        <h2>Available Hosts</h2>
        <span className="status-badge">
          <span className={`status-indicator ${status === 'connected' ? 'online' : 'offline'}`} />
          {status}
        </span>
      </div>

      {error && <div className="error-msg">{error}</div>}

      <div className="host-grid">
        {hosts.length === 0 && (
          <div className="empty-state">
            <span className="empty-icon">⎈</span>
            <p>No hosts online</p>
            <p className="hint">Start a host: <code>remotty host</code></p>
          </div>
        )}

        {hosts.map(host => (
          <div key={host.id} className="host-card" onClick={() => handleConnect(host)}>
            <div className="host-card-header">
              <span className="host-status-dot" />
              <span className="host-name">{host.name}</span>
              <span style={{ flex: 1 }} />
              <button
                className="btn-small"
                onClick={(e) => { e.stopPropagation(); setQrHost(qrHost?.id === host.id ? null : host); }}
                title="Show QR code"
              >
                qr
              </button>
            </div>
            <div className="host-card-body">
              <div className="host-detail">
                <span className="detail-label">Platform</span>
                <span className="detail-value">{host.platform}/{host.arch}</span>
              </div>
              <div className="host-detail">
                <span className="detail-label">Features</span>
                <span className="detail-value">{host.features?.join(', ') || 'terminal'}</span>
              </div>
            </div>
            {qrHost?.id === host.id && (
              <div style={{ marginTop: 12, padding: 12, background: '#fff', borderRadius: 8 }}>
                <QRCode
                  data={`remotty://connect/${encodeURIComponent(JSON.stringify({
                    signal: client?.getUrl() || '',
                    host: host.id,
                    name: host.name,
                  }))}`}
                  size={180}
                />
                <p style={{ textAlign: 'center', fontSize: 10, color: '#666', marginTop: 4 }}>
                  Scan with remotty app
                </p>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
