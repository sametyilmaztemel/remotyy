// Protocol types matching the Go backend

export type MessageType =
  | 'register' | 'heartbeat' | 'update'
  | 'list_hosts' | 'connect'
  | 'host_list' | 'room_ready' | 'peer_left'
  | 'offer' | 'answer' | 'ice_candidate'
  | 'auth' | 'auth_ok' | 'auth_fail'
  | 'input' | 'output' | 'resize'
  | 'screen_start' | 'screen_stop' | 'screen_frame' | 'screen_resize'
  | 'mouse_move' | 'mouse_click' | 'mouse_scroll'
  | 'key_press' | 'key_release'
  | 'file_request' | 'file_accept' | 'file_reject'
  | 'file_chunk' | 'file_complete' | 'file_progress'
  | 'file_cancel'
  | 'clipboard'
  | 'clipboard_data' | 'clipboard_request'
  | 'ping' | 'pong'
  | 'error';

export interface Message<T = unknown> {
  type: MessageType;
  payload?: T;
  from?: string;
  to?: string;
  room?: string;
  id?: string;
}

export interface HostInfo {
  id: string;
  name: string;
  platform: string;
  arch: string;
  version?: string;
  online: boolean;
  features: string[];
}

export interface ResizePayload {
  rows: number;
  cols: number;
}

export interface AuthPayload {
  password: string;
}

export interface FileRequestPayload {
  transfer_id: string;
  name: string;
  size: number;
  mime_type: string;
  chunk_size: number;
}

export interface FileChunkPayload {
  transfer_id: string;
  index: number;
  data: string; // base64-encoded chunk bytes (matches Go's json:\"data\" for []byte)
  checksum?: string;
}

export interface FileProgressPayload {
  transfer_id: string;
  bytes_sent: number;
  total_bytes: number;
  speed: number;
}

// ─── Screen Share (Data Channel) ─────────────────

export interface ScreenFramePayload {
  width: number;
  height: number;
  data: string; // base64-encoded JPEG bytes
}

export interface MouseMovePayload {
  x: number;
  y: number;
}

export interface MouseClickPayload {
  button: number;
  x: number;
  y: number;
  down: boolean;
}

export interface MouseScrollPayload {
  delta_x: number;
  delta_y: number;
}

export interface KeyPressPayload {
  key_code: number;
  chars: string;
  down: boolean;
}

export type ScreenDCMessage =
  | { type: 'screen_frame'; payload: ScreenFramePayload }
  | { type: 'mouse_move'; payload: MouseMovePayload }
  | { type: 'mouse_click'; payload: MouseClickPayload }
  | { type: 'mouse_scroll'; payload: MouseScrollPayload }
  | { type: 'key_press'; payload: KeyPressPayload }
  | { type: 'key_release'; payload: KeyPressPayload };

// ─── Clipboard ─────────────────────────────────────

export interface ClipboardPayload {
  text: string;
}

export interface ClipboardDataPayload {
  clipboard_text: string;
  timestamp?: number;
}

export interface ClipboardRequestPayload {
  request_id?: string;
}
