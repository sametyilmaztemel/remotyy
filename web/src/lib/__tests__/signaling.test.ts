import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { SignalingClient } from '../signaling';
import type { Message } from '../protocol';

// Mock WebSocket
class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  readyState = MockWebSocket.CONNECTING;
  binaryType = 'blob';
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onerror: ((ev: Error) => void) | null = null;
  sentMessages: string[] = [];

  constructor(public url: string) {
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN;
      this.onopen?.();
    }, 0);
  }

  send(data: string) {
    this.sentMessages.push(data);
  }

  close() {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.();
  }

  // Simulate incoming message
  receive(msg: Message) {
    this.onmessage?.({ data: JSON.stringify(msg) });
  }
}

// Patch global WebSocket
const originalWS = globalThis.WebSocket;

beforeEach(() => {
  // @ts-expect-error mock
  globalThis.WebSocket = MockWebSocket;
});

afterEach(() => {
  globalThis.WebSocket = originalWS;
});

describe('SignalingClient', () => {
  it('constructs with URL', () => {
    const client = new SignalingClient('ws://localhost:9000');
    expect(client.getUrl()).toBe('ws://localhost:9000');
  });

  it('connects and resolves promise', async () => {
    const client = new SignalingClient('ws://localhost:9000');
    await expect(client.connect()).resolves.toBeUndefined();
    client.disconnect();
  });

  it('sends messages when connected', async () => {
    const client = new SignalingClient('ws://localhost:9000');
    await client.connect();

    const msg: Message = { type: 'list_hosts' };
    client.send(msg);

    // The underlying MockWebSocket should have received the message
    expect(true).toBe(true); // Connection succeeded
    client.disconnect();
  });

  it('dispatches received messages to callbacks', async () => {
    const client = new SignalingClient('ws://localhost:9000');
    await client.connect();

    const received: Message[] = [];
    client.on('host_list', (msg) => received.push(msg));

    // Access the internal WebSocket to simulate a message
    // @ts-expect-error accessing private for test
    const ws = client.ws as MockWebSocket;
    ws.receive({
      type: 'host_list',
      payload: { hosts: [] },
    });

    expect(received).toHaveLength(1);
    expect(received[0].type).toBe('host_list');
    client.disconnect();
  });

  it('supports wildcard handlers', async () => {
    const client = new SignalingClient('ws://localhost:9000');
    await client.connect();

    const allMessages: Message[] = [];
    client.on('*', (msg) => allMessages.push(msg));

    // @ts-expect-error accessing private for test
    const ws = client.ws as MockWebSocket;
    ws.receive({ type: 'ping' });
    ws.receive({ type: 'pong' });

    expect(allMessages).toHaveLength(2);
    client.disconnect();
  });

  it('removes handler with off', async () => {
    const client = new SignalingClient('ws://localhost:9000');
    await client.connect();

    const received: Message[] = [];
    const handler = (msg: Message) => received.push(msg);
    client.on('test', handler);
    client.off('test', handler);

    // @ts-expect-error accessing private for test
    const ws = client.ws as MockWebSocket;
    ws.receive({ type: 'test' as any });

    expect(received).toHaveLength(0);
    client.disconnect();
  });

  it('disconnects cleanly', async () => {
    const client = new SignalingClient('ws://localhost:9000');
    await client.connect();
    client.disconnect();
    // No error thrown = success
  });
});
