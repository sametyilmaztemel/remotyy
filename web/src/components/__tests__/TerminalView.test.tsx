import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import TerminalView from '../TerminalView';
import type { HostInfo } from '../../lib/protocol';
import { useWebRTC } from '../../hooks/useWebRTC';

// ── Mocks ──────────────────────────────────────────────

// Mock xterm — must use regular function (not arrow) so it works with `new`
const mockTerminal = {
  open: vi.fn(),
  onData: vi.fn(),
  onResize: vi.fn(),
  loadAddon: vi.fn(),
  focus: vi.fn(),
  write: vi.fn(),
  dispose: vi.fn(),
  clear: vi.fn(),
  cols: 80,
  rows: 24,
};

vi.mock('xterm', () => ({
  Terminal: vi.fn(function () {
    return { ...mockTerminal };
  }),
}));

// Mock xterm-addon-fit
const mockFitAddon = {
  fit: vi.fn(),
  dispose: vi.fn(),
};

vi.mock('xterm-addon-fit', () => ({
  FitAddon: vi.fn(function () {
    return { ...mockFitAddon };
  }),
}));

// Mock xterm-addon-web-links
vi.mock('xterm-addon-web-links', () => ({
  WebLinksAddon: vi.fn(function () {
    return {};
  }),
}));

// Mock useSignaling hook
const mockClient = {
  send: vi.fn(),
  on: vi.fn(),
  off: vi.fn(),
  connect: vi.fn(),
  disconnect: vi.fn(),
};

vi.mock('../../hooks/useSignaling', () => ({
  useSignaling: vi.fn(() => ({
    client: mockClient,
    status: 'disconnected',
    connect: vi.fn(),
    disconnect: vi.fn(),
  })),
}));

// Mock useWebRTC hook
const mockWebRTC = {
  init: vi.fn(() => Promise.resolve()),
  close: vi.fn(),
  send: vi.fn(),
  connected: false,
  state: 'new',
  rtc: { current: null },
};

vi.mock('../../hooks/useWebRTC', () => ({
  useWebRTC: vi.fn(() => mockWebRTC),
}));

// Mock ResizeObserver
const mockResizeObserver = {
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
};

const MockResizeObserverImpl = vi.fn(function () {
  return mockResizeObserver;
});
vi.stubGlobal('ResizeObserver', MockResizeObserverImpl);

// ── Fixtures ───────────────────────────────────────────

const hostFixture: HostInfo = {
  id: 'h-001',
  name: 'test-server',
  platform: 'linux',
  arch: 'arm64',
  online: true,
  features: ['terminal'],
};

// ── Tests ──────────────────────────────────────────────

describe('TerminalView', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  // ── Render tests ─────────────────────────────────

  it('renders host name and status', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    expect(screen.getByText('test-server')).toBeInTheDocument();
    expect(screen.getByText('connecting...')).toBeInTheDocument();
  });

  it('renders Fit and Clear buttons', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    expect(screen.getByText('Fit')).toBeInTheDocument();
    expect(screen.getByText('Clear')).toBeInTheDocument();
  });

  it('renders terminal container div', () => {
    const { container } = render(
      <TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    const terminalContainer = container.querySelector('.terminal-container');
    expect(terminalContainer).toBeInTheDocument();
  });

  it('renders the terminal wrapper with class terminal-screen', () => {
    const { container } = render(
      <TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    const wrapper = container.querySelector('.terminal-screen');
    expect(wrapper).toBeInTheDocument();
  });

  // ── Terminal initialization ──────────────────────

  it('loads FitAddon and WebLinksAddon on mount', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    // loadAddon should have been called twice (FitAddon + WebLinksAddon)
    expect(mockTerminal.loadAddon).toHaveBeenCalledTimes(2);
  });

  it('opens terminal in the container div', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    expect(mockTerminal.open).toHaveBeenCalledTimes(1);
    expect(mockTerminal.focus).toHaveBeenCalledTimes(1);
  });

  it('registers ResizeObserver on terminal container', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    expect(MockResizeObserverImpl).toHaveBeenCalledTimes(1);
    expect(mockResizeObserver.observe).toHaveBeenCalledTimes(1);
  });

  // ── Button actions ───────────────────────────────

  it('calls fitAddon.fit when Fit button is clicked', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    // initial fit was called once during mount
    expect(mockFitAddon.fit).toHaveBeenCalledTimes(1);

    // Click Fit button
    fireEvent.click(screen.getByText('Fit'));
    expect(mockFitAddon.fit).toHaveBeenCalledTimes(2);
  });

  it('calls terminal.clear when Clear button is clicked', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    fireEvent.click(screen.getByText('Clear'));
    expect(mockTerminal.clear).toHaveBeenCalledTimes(1);
  });

  // ── Data flow ────────────────────────────────────

  it('sends data through WebRTC when terminal onData fires', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    // Simulate user typing via the onData callback captured by the component
    const onDataCallback = mockTerminal.onData.mock.calls[0][0];
    onDataCallback('ls -la\n');

    expect(mockWebRTC.send).toHaveBeenCalledWith('terminal', 'ls -la\n');
  });

  it('writes incoming data to terminal on data channel message', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    // Get the onDataChannel from the first call to useWebRTC
    const useWebRTCMock = vi.mocked(useWebRTC);
    const options = (useWebRTCMock as any).mock.calls[0][0];
    expect(options.onDataChannel).toBeDefined();

    // Simulate a data channel being opened with label 'terminal'
    const mockDc = { onmessage: null as any, send: vi.fn() };
    options.onDataChannel('terminal', mockDc);

    // Fire incoming data
    mockDc.onmessage({ data: 'Hello from host' });
    expect(mockTerminal.write).toHaveBeenCalledWith('Hello from host');
  });

  it('handles ArrayBuffer data channel messages by decoding to string', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    const useWebRTCMock = vi.mocked(useWebRTC);
    const options = (useWebRTCMock as any).mock.calls[0][0];
    const mockDc = { onmessage: null as any, send: vi.fn() };
    options.onDataChannel('terminal', mockDc);

    // Simulate incoming ArrayBuffer data
    const encoder = new TextEncoder();
    const buffer = encoder.encode('binary data').buffer;
    mockDc.onmessage({ data: buffer });

    expect(mockTerminal.write).toHaveBeenCalledWith('binary data');
  });

  // ── Lifecycle ────────────────────────────────────

  it('initializes WebRTC as offerer on mount', () => {
    render(<TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />);

    expect(mockWebRTC.init).toHaveBeenCalledWith(true);
  });

  it('disconnects ResizeObserver, WebRTC, and terminal on unmount', () => {
    const { unmount } = render(
      <TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    unmount();

    expect(mockResizeObserver.disconnect).toHaveBeenCalledTimes(1);
    expect(mockWebRTC.close).toHaveBeenCalledTimes(1);
    expect(mockTerminal.dispose).toHaveBeenCalledTimes(1);
  });

  it('stops writing to terminal after unmount (no crash)', () => {
    const { unmount } = render(
      <TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    const useWebRTCMock = vi.mocked(useWebRTC);
    const options = (useWebRTCMock as any).mock.calls[0][0];
    const mockDc = { onmessage: null as any, send: vi.fn() };
    options.onDataChannel('terminal', mockDc);

    unmount();

    // After unmount, writing to the terminal should fail silently (disposed)
    expect(() => {
      mockDc.onmessage({ data: 'after unmount' });
    }).not.toThrow();
  });

  // ── UI structure ─────────────────────────────────

  it('shows status indicator span with online class', () => {
    const { container } = render(
      <TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    const statusIndicator = container.querySelector('.status-indicator.small.online');
    expect(statusIndicator).toBeInTheDocument();
  });

  it('shows host name in the title section', () => {
    const { container } = render(
      <TerminalView host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    const titleDiv = container.querySelector('.terminal-title');
    expect(titleDiv).toBeInTheDocument();
    expect(titleDiv?.textContent).toContain('test-server');
  });
});
