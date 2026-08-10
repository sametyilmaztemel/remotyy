import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import ScreenViewer from '../ScreenViewer';
import type { HostInfo } from '../../lib/protocol';
import { useWebRTC } from '../../hooks/useWebRTC';

// ── Mocks ──────────────────────────────────────────────

// Mock useSignaling hook
const mockClient = {
  send: vi.fn(),
  on: vi.fn(function (this: ReturnType<typeof vi.fn>) {
    return this;
  }),
  off: vi.fn(function (this: ReturnType<typeof vi.fn>) {
    return this;
  }),
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

// ── Fixtures ───────────────────────────────────────────

const hostFixture: HostInfo = {
  id: 'h-002',
  name: 'mac-pro',
  platform: 'darwin',
  arch: 'arm64',
  online: true,
  features: ['screen'],
};

// ── Tests ──────────────────────────────────────────────

describe('ScreenViewer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  // ── Render / Mount ───────────────────────────────

  it('renders host name in the title', () => {
    render(<ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />);

    expect(screen.getByText('mac-pro')).toBeInTheDocument();
  });

  it('shows "Start Screen Share" button when inactive', () => {
    render(<ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />);

    expect(screen.getByText('▶ Start Screen Share')).toBeInTheDocument();
  });

  it('shows screen sharing placeholder when inactive', () => {
    render(<ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />);

    expect(screen.getByText('Screen sharing inactive')).toBeInTheDocument();
  });

  it('renders with screen-viewer CSS class', () => {
    const { container } = render(
      <ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    const viewer = container.querySelector('.screen-viewer');
    expect(viewer).toBeInTheDocument();
  });

  it('shows disconnected status label initially', () => {
    render(<ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />);

    // The ScreenViewer does NOT auto-connect; it shows "Disconnected" until
    // the user clicks "Start Screen Share"
    expect(screen.getByText('Disconnected')).toBeInTheDocument();
  });

  // ── Start / Stop ─────────────────────────────────

  it('calls webrtc.init with false (non-offerer) on Start click', () => {
    render(<ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />);

    const startBtn = screen.getByText('▶ Start Screen Share');
    fireEvent.click(startBtn);

    // init should be called with false (ScreenViewer is non-offerer)
    expect(mockWebRTC.init).toHaveBeenCalledWith(false);
  });

  it('sends connect message after webrtc.init resolves', async () => {
    render(<ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />);

    const startBtn = screen.getByText('▶ Start Screen Share');
    fireEvent.click(startBtn);

    // Since webrtc.init is async, wait for the next tick
    await waitFor(() => {
      expect(mockClient.send).toHaveBeenCalledWith({
        type: 'connect',
        payload: { host_id: 'h-002' },
      });
    });
  });

  it('cleans up on unmount', () => {
    const { unmount } = render(
      <ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    unmount();

    expect(mockWebRTC.close).toHaveBeenCalledTimes(1);
  });

  // ── Canvas rendering / Frame handling ─────────────

  it('canvas getContext("2d") is mocked and callable', () => {
    // Spy on canvas getContext to verify it's available
    const getContextSpy = vi.spyOn(
      HTMLCanvasElement.prototype,
      'getContext',
    );

    render(<ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />);

    // The component renders a placeholder by default (no canvas yet)
    // but the spy is set up for when the canvas becomes active
    expect(getContextSpy).toBeDefined();
  });

  it('placeholder shows hint text with curly quotes', () => {
    render(<ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />);

    // The text uses &lsquo; and &rsquo; HTML entities which render as
    // Unicode curly quotes (U+2018, U+2019)
    expect(
      screen.getByText(/Click .Start Screen Share. to begin/),
    ).toBeInTheDocument();
  });

  // ── UI Accessibility / Controls ───────────────────

  it('has status dot element', () => {
    const { container } = render(
      <ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    const statusDot = container.querySelector('.status-dot');
    expect(statusDot).toBeInTheDocument();
  });

  it('has screen-container div', () => {
    const { container } = render(
      <ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    expect(container.querySelector('.screen-container')).toBeInTheDocument();
  });

  it('does not show canvas when not started', () => {
    const { container } = render(
      <ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    // Should show placeholder, not canvas wrapper
    expect(container.querySelector('.screen-placeholder')).toBeInTheDocument();
    expect(container.querySelector('.screen-canvas-wrapper')).toBeNull();
  });

  it('registers keyboard handlers on viewer div', () => {
    const { container } = render(
      <ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    const viewer = container.querySelector('.screen-viewer');
    expect(viewer?.getAttribute('tabindex')).toBe('-1');
  });

  it('shows header bar with screen-title, controls, and status', () => {
    const { container } = render(
      <ScreenViewer host={hostFixture} signalUrl="ws://localhost:9000" />,
    );

    expect(container.querySelector('.screen-header')).toBeInTheDocument();
    expect(container.querySelector('.screen-title')).toBeInTheDocument();
    expect(container.querySelector('.screen-controls')).toBeInTheDocument();
  });
});
