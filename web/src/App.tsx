import { useState, useCallback } from 'react';
import { SignalingProvider } from './hooks/useSignaling';
import ConnectionForm from './components/ConnectionForm';
import HostList from './components/HostList';
import TerminalView from './components/TerminalView';
import ScreenViewer from './components/ScreenViewer';
import FileTransfer from './components/FileTransfer';
import ClipboardPanel from './components/ClipboardPanel';
import HelpView from './components/HelpView';
import type { HostInfo } from './lib/protocol';

type View = 'connect' | 'hosts' | 'terminal' | 'screen' | 'files' | 'clipboard' | 'help';

function App() {
  const [view, setView] = useState<View>('connect');
  const [signalUrl, setSignalUrl] = useState('ws://localhost:9000');
  const [selectedHost, setSelectedHost] = useState<HostInfo | null>(null);
  const [connected, setConnected] = useState(false);

  const handleConnect = useCallback((url: string) => {
    setSignalUrl(url);
    setView('hosts');
  }, []);

  const handleSelectHost = useCallback((host: HostInfo) => {
    setSelectedHost(host);
    setView('terminal');
  }, []);

  const handleDisconnect = useCallback(() => {
    setSelectedHost(null);
    setConnected(false);
    setView('connect');
  }, []);

  return (
    <SignalingProvider url={signalUrl}>
      <div className="app">
        <aside className="sidebar">
          <div className="sidebar-header">
            <span className="logo">⎈</span>
            <span className="logo-text">remotty</span>
          </div>
          <nav className="sidebar-nav">
            <button
              className={`nav-item ${view === 'connect' ? 'active' : ''}`}
              onClick={() => setView('connect')}
            >
              ⚡ Connect
            </button>
            <button
              className={`nav-item ${view === 'terminal' ? 'active' : ''}`}
              onClick={() => setView('terminal')}
              disabled={!selectedHost}
            >
              ⎇ Terminal
            </button>
            <button
              className={`nav-item ${view === 'screen' ? 'active' : ''}`}
              onClick={() => setView('screen')}
              disabled={!selectedHost}
            >
              🖵 Screen
            </button>
            <button
              className={`nav-item ${view === 'files' ? 'active' : ''}`}
              onClick={() => setView('files')}
              disabled={!selectedHost}
            >
              📁 Files
            </button>
            <button
              className={`nav-item ${view === 'clipboard' ? 'active' : ''}`}
              onClick={() => setView('clipboard')}
              disabled={!selectedHost}
            >
              📋 Clipboard
            </button>
          </nav>
          <div className="sidebar-footer">
            <button
              className={`nav-item ${view === 'help' ? 'active' : ''}`}
              onClick={() => setView('help')}
              style={{ marginBottom: 8 }}
            >
              ❓ Help
            </button>
            {selectedHost && (
              <div className="connected-info">
                <span className="status-dot" />
                {selectedHost.name}
              </div>
            )}
            <button className="disconnect-btn" onClick={handleDisconnect}>
              Disconnect
            </button>
          </div>
        </aside>

        <main className="main-content">
          {view === 'connect' && (
            <ConnectionForm onConnect={handleConnect} initialUrl={signalUrl} />
          )}
          {view === 'hosts' && (
            <HostList onSelect={handleSelectHost} />
          )}
          {view === 'terminal' && selectedHost && (
            <TerminalView host={selectedHost} signalUrl={signalUrl} />
          )}
          {view === 'screen' && selectedHost && (
            <ScreenViewer host={selectedHost} signalUrl={signalUrl} />
          )}
          {view === 'files' && selectedHost && (
            <FileTransfer host={selectedHost} signalUrl={signalUrl} />
          )}
          {view === 'clipboard' && selectedHost && (
            <ClipboardPanel host={selectedHost} />
          )}
          {view === 'help' && (
            <HelpView />
          )}
        </main>
      </div>
    </SignalingProvider>
  );
}

export default App;
