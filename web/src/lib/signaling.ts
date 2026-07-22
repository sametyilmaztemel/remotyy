import type { Message } from './protocol';

export type SignalCallback = (msg: Message) => void;

export class SignalingClient {
  private ws: WebSocket | null = null;
  private url: string;
  private callbacks: Map<string, SignalCallback[]> = new Map();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private maxReconnectAttempts = 5;
  private reconnectAttempts = 0;
  private onStatusChange: ((status: string) => void) | null = null;

  constructor(url: string) {
    this.url = url;
  }

  getUrl(): string {
    return this.url;
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(this.url + '/ws');
        this.ws.binaryType = 'arraybuffer';

        this.ws.onopen = () => {
          this.reconnectAttempts = 0;
          this.onStatusChange?.('connected');
          resolve();
        };

        this.ws.onmessage = (event) => {
          try {
            const msg: Message = JSON.parse(event.data);
            this.dispatch(msg);
          } catch (e) {
            console.error('Failed to parse message:', e);
          }
        };

        this.ws.onclose = () => {
          this.onStatusChange?.('disconnected');
          this.attemptReconnect();
        };

        this.ws.onerror = (err) => {
          this.onStatusChange?.('error');
          reject(err);
        };
      } catch (err) {
        reject(err);
      }
    });
  }

  send(msg: Message): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  on(type: string, cb: SignalCallback): void {
    if (!this.callbacks.has(type)) {
      this.callbacks.set(type, []);
    }
    this.callbacks.get(type)!.push(cb);
  }

  off(type: string, cb: SignalCallback): void {
    const cbs = this.callbacks.get(type);
    if (cbs) {
      const idx = cbs.indexOf(cb);
      if (idx >= 0) cbs.splice(idx, 1);
    }
  }

  onStatus(fn: (status: string) => void): void {
    this.onStatusChange = fn;
  }

  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
    }
    this.ws?.close();
    this.ws = null;
  }

  private dispatch(msg: Message): void {
    const cbs = this.callbacks.get(msg.type) || [];
    cbs.forEach(cb => cb(msg));
    // Also dispatch to wildcard handlers
    const wildcards = this.callbacks.get('*') || [];
    wildcards.forEach(cb => cb(msg));
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) return;
    this.reconnectAttempts++;
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
    this.onStatusChange?.(`reconnecting in ${delay / 1000}s...`);
    this.reconnectTimer = setTimeout(() => {
      this.connect().catch(() => {});
    }, delay);
  }
}
