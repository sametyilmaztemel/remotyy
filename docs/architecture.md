# Architecture

## Overview

remotty uses a **signaling server + WebRTC P2P** architecture:

1. **Host daemon** connects to signaling server via WebSocket, registers itself
2. **Client** connects to same signaling server, requests available hosts
3. Signaling server creates a **room** pairing host and client
4. **WebRTC handshake** happens through the signaling server (offer/answer/ICE)
5. Once P2P connection established, **terminal data flows directly** between devices
6. Signaling server only coordinates — **never sees terminal data**

## Data Flow

```
                  ┌──────────────────┐
                  │  Signaling       │
                  │  Server          │
                  │                  │
                  │  Host Registry   │
                  │  Room Manager    │
                  └──┬─────────────┬─┘
                     │             │
            ┌────────┘             └────────┐
            ▼                                ▼
    ┌───────────────┐              ┌──────────────────┐
    │  Host Daemon  │              │  Client           │
    │               │   WebRTC     │                   │
    │  ┌─────────┐  │◄─────────────│  ┌──────────────┐ │
    │  │ pty     │  │  DTLS-SRTP   │  │ xterm.js     │ │
    │  │ shell   │  │  DataChannel │  │ Terminal     │ │
    │  │ /bin/   │  │              │  │ Emulator     │ │
    │  │ bash    │  │              │  └──────────────┘ │
    │  └────┬────┘  │              │                   │
    │       │        │              │                   │
    │  ┌────▼────┐  │              │                   │
    │  │ WebRTC  │  │              │                   │
    │  │ Engine  │  │              │                   │
    │  └─────────┘  │              └──────────────────┘
    └───────────────┘
```

## Components

### 1. Signaling Server (`internal/signal/`)

**Role:** WebSocket hub that connects hosts with clients

- **Device Registry** — Tracks all connected hosts (hostname, platform, arch, features)
- **Room Manager** — Creates temporary rooms pairing one host + one client
- **Blind Relay** — Forwards WebRTC handshake messages (offer/answer/ICE) between peers
- **REST API** — `/health`, `/hosts` endpoints for monitoring
- **CORS** — Open CORS for web client access

**Protocol:** JSON over WebSocket

### 2. Host Daemon (`internal/host/`)

**Role:** Runs on the machine to be accessed remotely

- **Signal Connection** — Outbound WebSocket to signaling server
- **Heartbeat** — Periodic keepalive (15s)
- **Auto-reconnect** — Exponential backoff on disconnect
- **WebRTC Engine** — pion/webrtc peer connection management
- **PTY Manager** — Spawns and manages shell sessions via `creack/pty`
- **Auth** — Master password verification (bcrypt)
- **Device Allow List** — Optional client ID whitelist

### 3. Web Client (`web/`)

**Role:** Browser-based terminal emulator

- **xterm.js** — Full terminal emulator with themes and fit addon
- **WebRTC** — Browser native RTCPeerConnection
- **UI** — Dark theme, JetBrains Mono, responsive

### 4. Protocol (`internal/protocol/`)

**Message Types:**

| Type | Direction | Description |
|------|-----------|-------------|
| `register` | Host → Signal | Host announces itself |
| `heartbeat` | Host → Signal | Keepalive |
| `request_host` | Client → Signal | Ask for host list or specific host |
| `offer` | Bidirectional | WebRTC SDP offer |
| `answer` | Bidirectional | WebRTC SDP answer |
| `ice_candidate` | Bidirectional | ICE candidate for NAT traversal |
| `approved` | Signal → Client | Connection approved, room created |
| `auth` | Client → Host | Master password |
| `auth_ok` / `auth_fail` | Host → Client | Auth result |
| `input` | Client → Host | Terminal input |
| `output` | Host → Client | Terminal output |
| `resize` | Client → Host | Terminal resize |
| `error` | Any → Any | Error message |

## Security Model

```
Layer 1: Transport Security
├── WebRTC DTLS 1.3 — Datagram TLS for media/data channels
├── SRTP — Secure Real-time Transport Protocol
└── Perfect Forward Secrecy — Ephemeral key exchange

Layer 2: Signal Authentication
├── Bearer Token — REMOTTY_AUTH_TOKEN on signaling server
└── Token verification on connect

Layer 3: Device Authorization
├── Host maintain allow list of device IDs
├── Each client must be explicitly approved first time
└── Host reject unknown devices

Layer 4: Terminal Access
├── Master Password (bcrypt hashed)
├── Password never leaves host machine
└── Separate from signaling auth
```

## Deployment Topology

### Minimal (Single Machine)

```
┌─────────────────────────────────────┐
│  Single Server (ARM/Mac)             │
│  ┌──────────┐  ┌──────────┐         │
│  │ Signal   │  │ Host     │         │
│  │ :9000    │  │ Daemon   │         │
│  └──────────┘  └──────────┘         │
│         ┌──────────────┐             │
│         │ Web Client   │             │
│         │ :3000        │             │
│         └──────────────┘             │
└─────────────────────────────────────┘
```

### Distributed (Mac + ARM)

```
┌──────────────────┐     ┌──────────────────┐
│  ARM (Oracle)     │     │  Mac (Client)     │
│                   │     │                   │
│  ┌──────────────┐ │     │  ┌──────────────┐ │
│  │ Signal :9000 │ │     │  │ Web Client   │ │
│  │ Host Daemon  │ │     │  │ :3000        │ │
│  │ Hermes       │ │     │  │ Browser      │ │
│  └──────────────┘ │     │  └──────────────┘ │
└──────────────────┘     └──────────────────┘
         │                        │
         └──── WebRTC P2P ────────┘
```

### Production (ARM + CF Tunnel)

```
┌──────────┐    ┌──────────────┐    ┌──────────────────┐
│  Browser  │───▶│ Cloudflare   │───▶│  ARM Oracle       │
│  Client   │    │ Tunnel       │    │                   │
│           │    │ wss://...    │    │  Signal :9000     │
│           │    │              │    │  Host Daemon      │
│           │    │              │    │  Web :3000        │
│           │    │              │    │  Hermes Gateway   │
└──────────┘    └──────────────┘    └──────────────────┘
```
