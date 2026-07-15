import { useState, type FormEvent } from 'react';
import QRScanner from './QRScanner';

interface Props {
  onConnect: (url: string) => void;
  initialUrl: string;
}

export default function ConnectionForm({ onConnect, initialUrl }: Props) {
  const [url, setUrl] = useState(initialUrl);
  const [password, setPassword] = useState('');
  const [showQR, setShowQR] = useState(false);
  const [showHelp, setShowHelp] = useState(false);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (url.trim()) onConnect(url.trim());
  };

  const handleQRScan = (scannedUrl: string) => {
    // Parse remotty:// URL or use raw ws:// URL
    let signalUrl = scannedUrl;
    if (scannedUrl.startsWith('remotty://')) {
      // Extract signal URL from remotty:// protocol
      try {
        const payload = scannedUrl.replace('remotty://connect/', '');
        const data = JSON.parse(decodeURIComponent(payload));
        signalUrl = data.signal || scannedUrl;
      } catch {}
    }
    setUrl(signalUrl);
    setShowQR(false);
    // Auto-connect after QR scan
    if (signalUrl.trim()) onConnect(signalUrl.trim());
  };

  const detectLocalIP = async () => {
    try {
      const pc = new RTCPeerConnection();
      pc.createDataChannel('');
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      pc.addEventListener('icecandidate', (e) => {
        if (e.candidate) {
          const ip = e.candidate.candidate.split(' ')[4];
          if (ip && !ip.includes(':')) {
            setUrl(`ws://${ip}:9000`);
          }
          pc.close();
        }
      });
      setTimeout(() => pc.close(), 2000);
    } catch {}
  };

  return (
    <div className="connect-screen">
      <div className="connect-card">
        <div className="connect-logo">
          <span className="logo-icon">⎈</span>
          <h1>remotty</h1>
          <p className="tagline">remote terminal &middot; open source</p>
        </div>

        <form onSubmit={handleSubmit} className="connect-form">
          <div className="field">
            <label>Signaling Server</label>
            <div className="input-group">
              <input
                type="text"
                value={url}
                onChange={e => setUrl(e.target.value)}
                placeholder="ws://host:port"
                className="input mono"
              />
              <button type="button" className="btn-icon" onClick={detectLocalIP} title="Auto-detect IP">
                ↻
              </button>
              <button type="button" className="btn-icon" onClick={() => setShowQR(true)} title="Scan QR code">
                📷
              </button>
            </div>
          </div>

          <div className="field">
            <label>Master Password <span className="optional">(optional)</span></label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              placeholder="Leave blank if not set"
              className="input mono"
            />
          </div>

          <button type="submit" className="btn-primary">
            ⚡ Connect
          </button>
        </form>

        <div style={{ marginTop: 16, textAlign: 'center' }}>
          <button
            onClick={() => setShowHelp(!showHelp)}
            style={{
              background: 'none', border: 'none', color: 'var(--accent)',
              cursor: 'pointer', fontSize: 12, padding: 8,
            }}
          >
            {showHelp ? '▾ Hide guide' : '▸ How to connect'}
          </button>
        </div>

        {showHelp && (
          <div className="help-box">
            <strong>Quick start</strong>
            <ol>
              <li>Terminal: <code>remotty signal --dev</code></li>
              <li>Terminal: <code>remotty host --signal ws://localhost:9000 --qr</code></li>
              <li>Scan QR with phone camera (📷 button)</li>
              <li>Or use CLI: <code>remotty connect</code></li>
            </ol>
            <strong>Test from iPhone</strong>
            <ol>
              <li>Same WiFi required</li>
              <li>iPhone Safari: <code>http://&lt;MAC_IP&gt;:3000</code></li>
              <li>Enter <code>ws://&lt;MAC_IP&gt;:9000</code></li>
              <li>Connect → select host → terminal</li>
            </ol>
          </div>
        )}
      </div>

      {showQR && (
        <QRScanner onScan={handleQRScan} onClose={() => setShowQR(false)} />
      )}
    </div>
  );
}
