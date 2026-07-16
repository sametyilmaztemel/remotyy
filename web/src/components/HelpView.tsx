import { useState } from 'react';

export default function HelpView() {
  const [tab, setTab] = useState<'setup' | 'features' | 'about'>('setup');

  return (
    <div style={{ padding: 24, maxWidth: 640, margin: '0 auto', overflow: 'auto', height: '100%' }}>
      <div style={{ display: 'flex', gap: 8, marginBottom: 24, borderBottom: '1px solid var(--border)', paddingBottom: 8 }}>
        {(['setup', 'features', 'about'] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className="btn-small"
            style={{
              background: tab === t ? 'var(--accent-subtle)' : 'transparent',
              color: tab === t ? 'var(--accent)' : 'var(--text)',
              border: 'none', padding: '6px 12px', borderRadius: 4, cursor: 'pointer',
              fontFamily: 'var(--font-mono)', fontSize: 12,
            }}
          >
            {t === 'setup' ? '⚡ Setup' : t === 'features' ? '📋 Features' : 'ℹ️ About'}
          </button>
        ))}
      </div>

      {tab === 'setup' && (
        <div style={{ fontSize: 12, lineHeight: 1.7 }}>
          <h3 style={{ marginBottom: 12, color: 'var(--text-bright)' }}>Test between Mac and iPhone</h3>

          <div style={{ background: 'var(--bg-card)', borderRadius: 8, padding: 16, marginBottom: 16 }}>
            <strong>Same WiFi network</strong>
            <ol style={{ margin: '8px 0', paddingLeft: 20 }}>
              <li>Mac'te terminal açın</li>
              <li><code style={{ background: 'var(--bg)', padding: '1px 4px', borderRadius: 3 }}>remotty signal --dev</code></li>
              <li><code style={{ background: 'var(--bg)', padding: '1px 4px', borderRadius: 3 }}>remotty host --signal ws://localhost:9000</code></li>
              <li>Mac'in IP'sini bulun: <code style={{ background: 'var(--bg)', padding: '1px 4px', borderRadius: 3 }}>ipconfig getifaddr en0</code></li>
              <li>iPhone Safari: <code style={{ background: 'var(--bg)', padding: '1px 4px', borderRadius: 3 }}>http://&lt;MAC_IP&gt;:3000</code></li>
              <li>Signaling URL: <code style={{ background: 'var(--bg)', padding: '1px 4px', borderRadius: 3 }}>ws://&lt;MAC_IP&gt;:9000</code></li>
              <li>⚡ Connect → host'u seç → terminal başlasın</li>
            </ol>
          </div>

          <div style={{ background: 'var(--bg-card)', borderRadius: 8, padding: 16, marginBottom: 16 }}>
            <strong>Web server (if not running)</strong>
            <p style={{ margin: '6px 0' }}>
              <code style={{ background: 'var(--bg)', padding: '1px 4px', borderRadius: 3 }}>
                cd web && npx serve dist -l 3000
              </code>
            </p>
          </div>

          <div style={{ background: 'var(--bg-card)', borderRadius: 8, padding: 16 }}>
            <strong>From anywhere (internet)</strong>
            <p style={{ margin: '6px 0' }}>
              Signaling server'ı Cloudflare Tunnel arkasında çalıştırın.
              Host'u herhangi bir sunucuda başlatın.
              Web client aynı sunucuda serve edilsin.
            </p>
          </div>
        </div>
      )}

      {tab === 'features' && (
        <div style={{ fontSize: 12, lineHeight: 1.7 }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)' }}>
                <th style={{ textAlign: 'left', padding: 6, color: 'var(--text-dim)', fontWeight: 500 }}>Feature</th>
                <th style={{ textAlign: 'center', padding: 6, color: 'var(--text-dim)', fontWeight: 500 }}>Macky</th>
                <th style={{ textAlign: 'center', padding: 6, color: 'var(--text-dim)', fontWeight: 500 }}>remotty</th>
              </tr>
            </thead>
            <tbody>
              {[
                ['Terminal access', '✅', '✅'],
                ['Screen sharing', '✅', '🚧'],
                ['E2E encryption', '✅', '✅'],
                ['Zero open ports', '✅', '✅'],
                ['P2P &lt;50ms', '✅', '✅'],
                ['Web client', '❌', '✅'],
                ['CLI client', '❌', '✅'],
                ['Linux host', '❌', '✅'],
                ['Self-hosted', '❌', '✅'],
                ['Open source (MIT)', '❌', '✅'],
                ['File transfer', '❌', '🚧'],
                ['Port forwarding', '❌', '🚧'],
                ['iOS native app', '✅', '🚧'],
                ['macOS menu bar', '✅', '✅'],
                ['Free', '❌ ($29)', '✅'],
              ].map(([feature, macky, remotty]) => (
                <tr key={feature} style={{ borderBottom: '1px solid var(--border)' }}>
                  <td style={{ padding: '5px 6px' }}>{feature}</td>
                  <td style={{ padding: '5px 6px', textAlign: 'center' }}>{macky}</td>
                  <td style={{ padding: '5px 6px', textAlign: 'center' }}>{remotty}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {tab === 'about' && (
        <div style={{ fontSize: 12, lineHeight: 1.7, textAlign: 'center', paddingTop: 40 }}>
          <div style={{ fontSize: 48, marginBottom: 12 }}>⎈</div>
          <h2 style={{ color: 'var(--text-bright)', marginBottom: 4 }}>remotty</h2>
          <p style={{ color: 'var(--text-dim)', marginBottom: 16 }}>
            remote terminal &middot; open source
          </p>
          <p>Version 0.5.1</p>
          <p style={{ marginTop: 8 }}>
            <a href="https://github.com/remotty/remotty" style={{ color: 'var(--accent)' }}>
              github.com/remotty/remotty
            </a>
          </p>
        </div>
      )}
    </div>
  );
}
