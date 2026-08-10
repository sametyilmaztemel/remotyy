import { describe, it, expect } from 'vitest';
import type { Message, HostInfo, ResizePayload, FileRequestPayload } from '../protocol';

describe('protocol types', () => {
  it('Message has required type field', () => {
    const msg: Message = { type: 'register' };
    expect(msg.type).toBe('register');
    expect(msg.payload).toBeUndefined();
    expect(msg.from).toBeUndefined();
    expect(msg.room).toBeUndefined();
  });

  it('Message carries payload', () => {
    const msg: Message<{ name: string }> = {
      type: 'register',
      payload: { name: 'test-host' },
    };
    expect(msg.payload).toEqual({ name: 'test-host' });
  });

  it('Message serializes to valid JSON', () => {
    const msg: Message = {
      type: 'ice_candidate',
      payload: { candidate: 'test', sdpMid: '0' },
      room: 'r-123',
    };
    const json = JSON.stringify(msg);
    const parsed = JSON.parse(json);
    expect(parsed.type).toBe('ice_candidate');
    expect(parsed.room).toBe('r-123');
    expect(parsed.payload.candidate).toBe('test');
  });
});

describe('HostInfo', () => {
  it('has required fields', () => {
    const host: HostInfo = {
      id: 'h-001',
      name: 'test-server',
      platform: 'linux',
      arch: 'arm64',
      online: true,
      features: ['terminal', 'screen'],
    };
    expect(host.id).toBe('h-001');
    expect(host.online).toBe(true);
    expect(host.features).toHaveLength(2);
    expect(host.features).toContain('terminal');
  });

  it('optional version field', () => {
    const host: HostInfo = {
      id: 'h-002',
      name: 'mac-pro',
      platform: 'darwin',
      arch: 'arm64',
      online: false,
      features: ['terminal'],
      version: '0.7.1',
    };
    expect(host.version).toBe('0.7.1');
  });
});

describe('ResizePayload', () => {
  it('round-trips through JSON', () => {
    const resize: ResizePayload = { rows: 24, cols: 80 };
    const json = JSON.stringify(resize);
    const parsed: ResizePayload = JSON.parse(json);
    expect(parsed.rows).toBe(24);
    expect(parsed.cols).toBe(80);
  });
});

describe('FileRequestPayload', () => {
  it('includes all file transfer fields', () => {
    const req: FileRequestPayload = {
      transfer_id: 't-001',
      name: 'test.txt',
      size: 1024,
      mime_type: 'text/plain',
      chunk_size: 32768,
    };
    expect(req.transfer_id).toBe('t-001');
    expect(req.chunk_size).toBe(32768);
  });
});

describe('all message types', () => {
  const messageTypes = [
    'register', 'heartbeat', 'update',
    'list_hosts', 'connect',
    'host_list', 'room_ready', 'peer_left',
    'offer', 'answer', 'ice_candidate',
    'auth', 'auth_ok', 'auth_fail',
    'input', 'output', 'resize',
    'screen_start', 'screen_stop',
    'file_request', 'file_accept', 'file_reject',
    'file_chunk', 'file_complete', 'file_progress',
    'clipboard',
    'ping', 'pong',
    'error',
  ] as const;

  it.each(messageTypes)('type "%s" creates valid message', (type) => {
    const msg: Message = { type };
    const json = JSON.stringify(msg);
    const parsed = JSON.parse(json);
    expect(parsed.type).toBe(type);
  });
});
