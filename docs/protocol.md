# remotty Protocol Documentation

> **Version:** 0.2.0  
> **Last updated:** 2026-06-04  
> **Go module:** `github.com/sametyilmaztemel/remotty/internal/protocol`

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Transport Layers](#transport-layers)
3. [Message Envelope](#message-envelope)
4. [Signaling Protocol](#signaling-protocol)
5. [WebRTC Negotiation](#webrtc-negotiation)
6. [Data Channel Protocol](#data-channel-protocol)
7. [Message Type Reference](#message-type-reference)
8. [Connection Flow (Sequence Diagrams)](#connection-flow)
9. [iOS App Integration Notes](#ios-app-integration-notes)

---

## Architecture Overview

```
                    ┌──────────────────┐
                    │  Signaling Server│  (WebSocket relay)
                    │  /ws endpoint    │
                    └───────┬──────────┘
                            │ (blind relay — never sees data)
                  ┌─────────┴──────────┐
                  ▼                    ▼
          ┌──────────────┐    ┌──────────────────┐
          │  Host Daemon  │    │  Client(s)        │
          │  (Go binary)  │    │  Web / CLI / iOS  │
          │               │    │                   │
          │  pty → shell  │    │  xterm.js / SwiftUI│
          │  screen capt. │    │  (GoogleWebRTC)   │
          └──────┬───────-┘    └────────┬─────────-┘
                 │                      │
                 └────── WebRTC ────────┘
                 (DTLS-SRTP encrypted P2P)
```

**Key design principles:**

- **Blind signaling**: The signaling server coordinates the WebRTC handshake only. It never sees terminal data, screen frames, or any payload content. It relays `offer`, `answer`, and `ice_candidate` messages between peers.
- **Two message planes**: Signaling messages travel over a WebSocket to the server. Data channel messages travel over the WebRTC peer connection directly between host and client.
- **Envelope format**: Every message on both planes uses a consistent JSON envelope (`Message`).

---

## Transport Layers

### 1. Signaling Plane (WebSocket)

- **Endpoint:** `ws://<server>:<port>/ws`
- **Protocol:** Raw WebSocket (binary or text, JSON-encoded)
- **Max payload:** 1 MB (configurable in signal server)
- **Purpose:** Coordination — host registration, client discovery, room creation, SDP/ICE relay

**REST endpoints (auxiliary):**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/ws` | GET (upgrade) | WebSocket signaling |
| `/health` | GET | Health check → `{"status":"ok",...}` |
| `/api/hosts` | GET | List registered hosts (REST) |
| `/api/stats` | GET | Server metrics (peer count, room count) |

### 2. Data Plane (WebRTC Data Channels)

- **Transport:** WebRTC peer connection (pion/webrtc on Go, GoogleWebRTC on iOS, browser RTCPeerConnection on web)
- **Encryption:** DTLS-SRTP (mandatory in WebRTC — always E2E encrypted)
- **ICE servers:** `stun:stun.l.google.com:19302` (default)
- **Data channel config:** Ordered delivery, unlimited retransmits

**Data channels (by label):**

| Label | Purpose | Direction |
|-------|---------|-----------|
| `terminal` | PTY I/O: raw bytes + resize | Bidirectional |
| `screen` | Screen frames + remote input events | Host→Client (frames), Client→Host (input) |
| `auth` | Authentication handshake | Client→Host (auth), Host→Client (auth_ok/fail) |
| `file` | File transfers | Bidirectional |
| `clipboard` | Clipboard sync | Bidirectional |

---

## Message Envelope

Every message on both the signaling WebSocket and data channels shares this JSON envelope:

```json
{
  "type": "<message_type>",
  "payload": { ... },
  "from": "<peer_id>",
  "to": "<peer_id>",
  "room": "<room_id>",
  "id": "<uuid>",
  "time": 1717459200
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | **always** | Message type string (see [Message Type Reference](#message-type-reference)) |
| `payload` | object/string | _optional_ | Type-specific payload (see below) |
| `from` | string | signaling | Sender peer ID (set by server) |
| `to` | string | signaling | Target peer ID (set by server) |
| `room` | string | signaling | Room ID (set by server after room creation) |
| `id` | string | _optional_ | Deduplication UUID |
| `time` | int64 | _optional_ | Unix timestamp |

**Go struct** (`internal/protocol/message.go`):

```go
type Message struct {
    Type    MessageType     `json:"type"`
    Payload json.RawMessage `json:"payload,omitempty"`
    From    string          `json:"from,omitempty"`
    To      string          `json:"to,omitempty"`
    Room    string          `json:"room,omitempty"`
    ID      string          `json:"id,omitempty"`
    Time    int64           `json:"time,omitempty"`
}
```

---

## Signaling Protocol

### Message Types (Signaling)

These travel over the WebSocket between a peer (host or client) and the signaling server.

#### Host → Server

| Type | Direction | Payload | Description |
|------|-----------|---------|-------------|
| `register` | Host → Server | [`RegisterPayload`](#registerpayload) | Host announces itself on connect |
| `heartbeat` | Host → Server | _none_ | Keepalive (default: every 15s) |
| `update` | Host → Server | [`HostInfo`](#hostinfo) | Update capabilities/features |

#### Client → Server

| Type | Direction | Payload | Description |
|------|-----------|---------|-------------|
| `list_hosts` | Client → Server | _none_ | Request list of available hosts |
| `connect` | Client → Server | [`ConnectPayload`](#connectpayload) | Request connection to a specific host |

#### Server → Peer

| Type | Direction | Payload | Description |
|------|-----------|---------|-------------|
| `register` | Server → Host | `{"id":"p-...", "status":"ok"}` | Registration confirmation with assigned peer ID |
| `host_list` | Server → Client | `{"hosts": [HostInfo, ...]}` | Response to `list_hosts` |
| `room_ready` | Server → Client | `{"room":"r-...", "host_id":"p-...", "host":HostInfo}` | Room created, ready for WebRTC |
| `connect` | Server → Host | `{"room":"r-...", "client_id":"p-..."}` | Notify host of incoming client |
| `peer_left` | Server → Remaining Peer | `{"peer_id":"p-...", "reason":"disconnected"}` | Peer disconnect notification |
| `error` | Server → Peer | [`ErrorPayload`](#errorpayload) | Error response |

#### WebRTC Negotiation (relayed through server)

| Type | Direction | Payload | Description |
|------|-----------|---------|-------------|
| `offer` | Peer → Peer (via server) | SDP offer | WebRTC SDP offer |
| `answer` | Peer → Peer (via server) | SDP answer | WebRTC SDP answer |
| `ice_candidate` | Peer → Peer (via server) | ICE candidate | Trickle ICE candidate |
| `renegotiate` | Peer → Peer (via server) | _varies_ | Renegotiation request |

### Payload Definitions

#### RegisterPayload

Sent by a host immediately after WebSocket connection to announce itself.

```json
{
  "name": "my-macbook",
  "platform": "darwin",
  "arch": "arm64",
  "version": "0.2.0",
  "features": ["terminal", "screen", "clipboard"],
  "device_id": "abc123"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Human-readable host name (max 64 chars) |
| `platform` | string | yes | OS: `darwin`, `linux`, `windows` |
| `arch` | string | yes | CPU arch: `arm64`, `amd64` |
| `version` | string | yes | remotty version |
| `features` | []string | yes | Supported features: `terminal`, `screen`, `file`, `clipboard` (max 20) |
| `device_id` | string | no | Stable hardware identifier |

#### HostInfo

Describes a registered host (used in responses and updates).

```json
{
  "id": "p-1717459200123456789",
  "name": "my-macbook",
  "platform": "darwin",
  "arch": "arm64",
  "version": "0.2.0",
  "online": true,
  "features": ["terminal", "screen"],
  "device_id": "abc123",
  "ping": 12
}
```

#### ConnectPayload

Sent by a client to request a connection to a specific host.

```json
{
  "host_id": "p-1717459200123456789",
  "password": "optional-master-password"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `host_id` | string | yes | Host peer ID **or** host name (server tries both) |
| `password` | string | no | Master password (forwarded by server, verified by host) |

#### ErrorPayload

```json
{
  "code": 0,
  "message": "host_not_found: Host offline"
}
```

---

## WebRTC Negotiation

The WebRTC handshake uses the signalling server as a blind relay for SDP and ICE candidates.

### Negotiation Flow

```
CLIENT                     SIGNAL SERVER                 HOST
  │                              │                         │
  │  ─── connect ──────────────► │                         │
  │                              │  ─── connect ──────────►│
  │                              │                         │
  │                              │      [Create WebRTC engine]
  │                              │      [CreateOffer()]
  │                              │                         │
  │  ◄─── room_ready ─────────── │                         │
  │  [Create WebRTC engine]      │                         │
  │                              │                         │
  │  ◄───── offer ────────────── │ ◄────── offer ───────── │
  │                              │                         │
  │  [HandleOffer → CreateAnswer]│                         │
  │                              │                         │
  │  ────── answer ────────────► │ ────── answer ────────► │
  │                              │   [HandleAnswer]         │
  │                              │                         │
  │  ◄── ice_candidate ──────────│ ◄──── ice_candidate ────│
  │  ◄── ice_candidate ───────── │ ◄──── ice_candidate ─── │
  │  ───── ice_candidate ───────►│ ───── ice_candidate ───►│
  │                              │                         │
  │     [ICE connected]          │                         │
  │     [Data channels open]     │                         │
  │                              │                         │
  │  ── auth (on "auth" DC) ─────────────────────────────► │
  │  ◄─ auth_ok (on "auth" DC) ─────────────────────────── │
  │                              │                         │
  │  ── resize (on "terminal") ──────────────────────────► │
  │  ◄─ terminal output ────────────────────────────────── │
```

### Key Points

1. **The host is always the offerer.** The host creates the WebRTC offer in response to receiving a `connect` notification from the server.
2. **The client is always the answerer.** The client receives the offer and creates an answer.
3. **ICE candidates are trickled.** Both sides send candidates as they gather them, and the server relays them immediately.
4. **Data channels are created by the client** after the peer connection is established (via `CreateDataChannel` on the client side; the host receives them via `OnDataChannel`).

### Go Implementation

**Host side** (`internal/host/daemon.go`):
- On `connect`: creates `webrtc.Engine`, calls `CreateOffer()`, sends `offer` via signaling

**Client side** (`internal/client/client.go`):
- On `room_ready`: creates `webrtc.Engine`
- On `offer`: calls `engine.HandleOffer()` which sends back an `answer`
- On `ice_candidate`: calls `engine.HandleICE()`
- After setup: creates data channels (`terminal`, `auth`)

**Engine** (`internal/webrtc/engine.go`):
- `CreateOffer()`: Creates SDP offer, sets local description, returns SDP
- `HandleOffer()`: Sets remote description, creates answer, sets local description, sends answer via signaling
- `HandleAnswer()`: Sets remote description
- `HandleICE()`: Adds ICE candidate

---

## Data Channel Protocol

Once the WebRTC peer connection is established and data channels are open, all further communication happens directly between host and client via data channels.

### Common Pattern

On every data channel, messages follow the same envelope format as signaling:

```json
{
  "type": "message_type",
  "payload": { ... }
}
```

However, the **terminal channel has a fast path**: raw bytes (not wrapped in JSON) are written directly. The host checks if incoming data is valid JSON; if not, it treats it as raw terminal input.

### 1. Auth Channel (`label: "auth"`)

Used before terminal/screen access is granted.

**Client → Host:**

```json
{
  "type": "auth",
  "payload": { "password": "secret" }
}
```

**Host → Client:**

```json
{ "type": "auth_ok", "payload": null }
```

```json
{ "type": "auth_fail", "payload": null }
```

### 2. Terminal Channel (`label: "terminal"`)

Bidirectional terminal I/O.

**Client → Host:**

```json
// Fast path (raw bytes — recommended for performance):
<raw UTF-8 bytes of terminal input>

// JSON path (structured):
{ "type": "input", "payload": "ls -la\n" }

// Resize notification (JSON only):
{
  "type": "resize",
  "payload": { "rows": 24, "cols": 80 }
}
```

**Host → Client:**

```json
// Fast path (raw bytes — recommended):
<raw UTF-8 bytes of terminal output>

// JSON path (structured):
{ "type": "output", "payload": "ls -la\n" }
```

**Important:** The terminal channel uses a **fast path** — raw binary data is sent directly without JSON wrapping. Both sides should try to parse incoming data as JSON first, and if it fails, treat it as raw bytes. This is more efficient for high-throughput PTY I/O.

### 3. Screen Channel (`label: "screen"`)

Screen sharing frames and remote input events.

**Host → Client (screen frames):**

```json
{
  "type": "screen_frame",
  "payload": {
    "width": 1920,
    "height": 1080,
    "data": "<base64-encoded JPEG bytes>"
  }
}
```

```json
{
  "type": "screen_resize",
  "payload": {
    "width": 1920,
    "height": 1080
  }
}
```

**Client → Host (input events):**

```json
{
  "type": "mouse_move",
  "payload": { "x": 960.5, "y": 540.0 }
}
```

```json
{
  "type": "mouse_click",
  "payload": { "button": 0, "x": 960.5, "y": 540.0, "down": true }
}
```

```json
{
  "type": "mouse_scroll",
  "payload": { "delta_x": -10.0, "delta_y": 25.0 }
}
```

```json
{
  "type": "key_press",
  "payload": { "key_code": 36, "chars": "a" }
}
```

```json
{
  "type": "key_release",
  "payload": { "key_code": 36 }
}
```

**Client → Host (screen control):**

```json
{
  "type": "screen_start",
  "payload": { "fps": 15, "quality": 60, "max_dimension": 1920, "capture_cursor": true }
}
```

```json
{
  "type": "screen_stop",
  "payload": {}
}
```

### 4. File Channel (`label: "file"`)

Chunked file transfer with SHA-256 checksums.

```json
// Client requests a file:
{
  "type": "file_request",
  "payload": {
    "transfer_id": "uuid",
    "name": "report.pdf",
    "size": 1048576,
    "mime_type": "application/pdf",
    "chunk_size": 65536
  }
}

// Host accepts:
{ "type": "file_accept", "payload": { "transfer_id": "uuid" } }

// Host rejects:
{ "type": "file_reject", "payload": { "transfer_id": "uuid" } }

// Data chunk:
{
  "type": "file_chunk",
  "payload": {
    "transfer_id": "uuid",
    "index": 0,
    "data": [byte, byte, ...],
    "checksum": "sha256-of-chunk"
  }
}

// Progress update:
{
  "type": "file_progress",
  "payload": {
    "transfer_id": "uuid",
    "bytes_sent": 524288,
    "total_bytes": 1048576,
    "speed": 52428800
  }
}

// Transfer complete:
{ "type": "file_complete", "payload": { "transfer_id": "uuid" } }

// Cancel:
{ "type": "file_cancel", "payload": { "transfer_id": "uuid" } }
```

### 5. Clipboard Channel (`label: "clipboard"`)

```json
{
  "type": "clipboard",
  "payload": { "text": "copied content" }
}
```

### 6. Keepalive

```json
// Signaling plane:
{ "type": "ping" }
{ "type": "pong" }

// Data channel:
{ "type": "ping" }
{ "type": "pong" }
```

---

## Message Type Reference

### Complete List of All Message Types

| Constant (Go) | String Value | Plane | Direction |
|---------------|-------------|-------|-----------|
| `MsgRegister` | `"register"` | Signaling | Bi (host→ server, server→ host ack) |
| `MsgHeartbeat` | `"heartbeat"` | Signaling | Host → Server |
| `MsgUpdate` | `"update"` | Signaling | Host → Server |
| `MsgListHosts` | `"list_hosts"` | Signaling | Client → Server |
| `MsgConnect` | `"connect"` | Signaling | Client → Server / Server → Host |
| `MsgHostList` | `"host_list"` | Signaling | Server → Client |
| `MsgRoomReady` | `"room_ready"` | Signaling | Server → Client |
| `MsgPeerLeft` | `"peer_left"` | Signaling | Server → Remaining Peer |
| `MsgOffer` | `"offer"` | Signaling | Peer ↔ Peer (via server) |
| `MsgAnswer` | `"answer"` | Signaling | Peer ↔ Peer (via server) |
| `MsgICECandidate` | `"ice_candidate"` | Signaling | Peer ↔ Peer (via server) |
| `MsgRenegotiate` | `"renegotiate"` | Signaling | Peer → Peer |
| `MsgAuth` | `"auth"` | Data (auth DC) | Client → Host |
| `MsgAuthOK` | `"auth_ok"` | Data (auth DC) | Host → Client |
| `MsgAuthFail` | `"auth_fail"` | Data (auth DC) | Host → Client |
| `MsgInput` | `"input"` | Data (terminal DC) | Client → Host |
| `MsgOutput` | `"output"` | Data (terminal DC) | Host → Client |
| `MsgResize` | `"resize"` | Data (terminal DC) | Client → Host |
| `MsgScreenStart` | `"screen_start"` | Data (screen DC) | Client → Host |
| `MsgScreenStop` | `"screen_stop"` | Data (screen DC) | Client → Host |
| `MsgScreenFrame` | `"screen_frame"` | Data (screen DC) | Host → Client |
| `MsgScreenResize` | `"screen_resize"` | Data (screen DC) | Host → Client |
| `MsgMouseMove` | `"mouse_move"` | Data (screen DC) | Client → Host |
| `MsgMouseClick` | `"mouse_click"` | Data (screen DC) | Client → Host |
| `MsgMouseScroll` | `"mouse_scroll"` | Data (screen DC) | Client → Host |
| `MsgKeyPress` | `"key_press"` | Data (screen DC) | Client → Host |
| `MsgKeyRelease` | `"key_release"` | Data (screen DC) | Client → Host |
| `MsgFileRequest` | `"file_request"` | Data (file DC) | Client → Host |
| `MsgFileAccept` | `"file_accept"` | Data (file DC) | Host → Client |
| `MsgFileReject` | `"file_reject"` | Data (file DC) | Host → Client |
| `MsgFileChunk` | `"file_chunk"` | Data (file DC) | Bidirectional |
| `MsgFileComplete` | `"file_complete"` | Data (file DC) | Bidirectional |
| `MsgFileProgress` | `"file_progress"` | Data (file DC) | Bidirectional |
| `MsgFileCancel` | `"file_cancel"` | Data (file DC) | Bidirectional |
| `MsgClipboard` | `"clipboard"` | Data (clipboard DC) | Bidirectional |
| `MsgPing` | `"ping"` | Signaling + Data | Bidirectional |
| `MsgPong` | `"pong"` | Signaling + Data | Bidirectional |
| `MsgError` | `"error"` | Signaling + Data | Server → Peer / Host → Client |

---

## Connection Flow

### Full Connection Lifecycle

```
PHASE 1: HOST REGISTRATION
───────────────────────────

Host                        Signal Server
  │                               │
  ├── WebSocket connect ─────────►│
  │                               │
  ├── MsgRegister ───────────────►│
  │   {                            │
  │     "type":"register",         │
  │     "payload":{                │
  │       "name":"my-host",        │
  │       "platform":"darwin",     │
  │       "arch":"arm64",          │
  │       "version":"0.2.0",       │
  │       "features":["terminal"]  │
  │     }                          │
  │   }                            │
  │                               │
  │◄── MsgRegister (ack) ────────│
  │   {                            │
  │     "type":"register",         │
  │     "payload":{                │
  │       "id":"p-...",            │
  │       "status":"ok"            │
  │     }                          │
  │   }                            │
  │                               │
  ├── MsgHeartbeat (every 15s) ──►│
  │   { "type":"heartbeat" }       │
  │                               │
  │  [Host is now available]      │


PHASE 2: CLIENT DISCOVERY
─────────────────────────

Client                      Signal Server
  │                               │
  ├── WebSocket connect ─────────►│
  │                               │
  ├── MsgListHosts ──────────────►│
  │   { "type":"list_hosts" }     │
  │                               │
  │◄── MsgHostList ──────────────│
  │   {                           │
  │     "type":"host_list",       │
  │     "payload":{               │
  │       "hosts":[               │
  │         {"id":"p-...","name":"my-host",...},
  │         ...
  │       ]                        │
  │     }                          │
  │   }                            │
  │                                │
  │◄── WebSocket close ──────────│
  │  [Short-lived: open→list→close]


PHASE 3: CONNECT TO HOST
────────────────────────

Client                      Signal Server                  Host
  │                               │                         │
  ├── WebSocket connect ─────────►│                         │
  │                               │                         │
  ├── MsgConnect ────────────────►│                         │
  │   {                            │                         │
  │     "type":"connect",          │                         │
  │     "payload":{                │                         │
  │       "host_id":"p-..."       │                         │
  │     }                          │                         │
  │   }                            │                         │
  │                               │                         │
  │      [Server creates room]    │                         │
  │      [Marks both as InRoom]   │                         │
  │                               │                         │
  │◄── MsgRoomReady ────────────│                         │
  │   {                            │                         │
  │     "type":"room_ready",       │                         │
  │     "payload":{                │                         │
  │       "room":"r-...",          │                         │
  │       "host_id":"p-...",       │                         │
  │       "host":{...}             │                         │
  │     }                          │                         │
  │   }                            │                         │
  │                               │                         │
  │                               ├── MsgConnect ──────────►│
  │                               │   {                      │
  │                               │     "type":"connect",    │
  │                               │     "payload":{          │
  │                               │       "room":"r-...",    │
  │                               │       "client_id":"p-.." │
  │                               │     }                    │
  │                               │   }                      │
  │                               │                         │
  │                               │    [Host creates         │
  │                               │     WebRTC engine]       │
  │                               │    [Host creates offer]  │
  │                               │                         │
  │                               │◄── MsgOffer ───────────│
  │◄── relay(MsgOffer) ──────────│                         │
  │                               │                         │
  │  [Client creates WebRTC eng.] │                         │
  │  [Client sets remote desc]    │                         │
  │  [Client creates answer]      │                         │
  │                               │                         │
  ├── relay(MsgAnswer) ──────────►├── MsgAnswer ───────────►│
  │                               │                         │
  │  [Trickle ICE candidates both ways via relay]            │
  │                               │                         │
  │  [ICE connection established] │                         │
  │  [Data channels open]         │                         │
  │                               │                         │


PHASE 4: DATA CHANNEL COMMUNICATION
──────────────────────────────────

Client (iOS/Web)                                     Host
  │                                                    │
  │  [Create "terminal" data channel]                  │
  │  [Create "auth" data channel]                      │
  │                                                    │
  ├── MsgAuth (on "auth" DC) ────────────────────────►│
  │   { "type":"auth", "payload":{"password":"..."} }   │
  │                                                    │
  │  [Host validates password]                         │
  │                                                    │
  │◄── MsgAuthOK (on "auth" DC) ──────────────────────│
  │   { "type":"auth_ok" }                              │
  │                                                    │
  │  [Session authenticated]                           │
  │                                                    │
  ├── MsgResize (on "terminal" DC) ───────────────────►│
  │   { "type":"resize","payload":{"rows":24,"cols":80}}│
  │                                                    │
  │  [Host spawns PTY shell]                           │
  │                                                    │
  ├── raw bytes (terminal input) ────────────────────►│
  │   [typed characters, fast path]                     │
  │                                                    │
  │◄── raw bytes (terminal output) ───────────────────│
  │   [shell output, fast path]                        │
  │                                                    │
  │  [Optional: screen sharing]                        │
  │                                                    │
  ├── MsgScreenStart ─────────────────────────────────►│
  │   { "type":"screen_start","payload":{"fps":15,...}}│
  │                                                    │
  │◄── MsgScreenFrame (repeating) ────────────────────│
  │   { "type":"screen_frame",                         │
  │     "payload":{"width":1920,"height":1080,          │
  │                "data":"<base64 JPEG>"}}             │
  │                                                    │
  ├── MsgMouseMove ───────────────────────────────────►│
  ├── MsgMouseClick ──────────────────────────────────►│
  └── MsgMouseScroll ─────────────────────────────────►│
```

### Peer State Machine (Server Side)

```
                ┌─────────────┐
                │  PeerNew    │
                └──────┬──────┘
                       │
          ┌────────────┼────────────┐
          │ register   │            │ list_hosts / connect
          ▼            │            │
   ┌─────────────┐     │            ▼
   │ PeerRegistered│    │   ┌─────────────┐
   └──────┬──────┘     │   │ PeerRegistered│
          │            │   └──────┬──────┘
          │ connect    │          │ room created
          │ (via room) │          ▼
          └────────────┴───>┌──────────┐
                            │ PeerInRoom│
                            └────┬─────┘
                                 │ disconnect
                                 ▼
                          ┌───────────────┐
                          │PeerDisconnected│
                          └───────────────┘
```

---

## Payload JSON Schemas

### Terminal

**ResizePayload:**
```json
{
  "rows": 24,
  "cols": 80
}
```

### Screen

**ScreenConfigPayload:**
```json
{
  "fps": 15,
  "quality": 60,
  "max_dimension": 1920,
  "capture_cursor": true
}
```

**ScreenFramePayload (on data channel):**
```json
{
  "width": 1920,
  "height": 1080,
  "data": "<base64-encoded JPEG bytes>"
}
```

### Input Events

**MouseMovePayload:**
```json
{ "x": 960.5, "y": 540.0 }
```

**MouseClickPayload:**
```json
{ "button": 0, "x": 960.5, "y": 540.0, "down": true }
```
`button`: 0=left, 1=right, 2=middle

**MouseScrollPayload:**
```json
{ "delta_x": -10.0, "delta_y": 25.0 }
```

**KeyPayload:**
```json
{ "key_code": 36, "chars": "a" }
```

### File Transfer

**FileRequestPayload:**
```json
{
  "transfer_id": "uuid-v4",
  "name": "filename.ext",
  "size": 1048576,
  "mime_type": "application/pdf",
  "chunk_size": 65536
}
```

**FileChunkPayload:**
```json
{
  "transfer_id": "uuid-v4",
  "index": 0,
  "data": [byte, byte, ...],
  "checksum": "sha256-hex-of-chunk-data"
}
```

### Clipboard

**ClipboardPayload:**
```json
{ "text": "copied text content" }
```

### Auth

**AuthPayload:**
```json
{ "password": "plaintext-master-password" }
```
The password is verified by the host against its bcrypt hash.

### Error

**ErrorPayload:**
```json
{ "code": 0, "message": "descriptive error string" }
```

---

## Host Daemon Internal Architecture

### Session Lifecycle

```
1. Signal server notifies host of incoming client (MsgConnect)
2. Host creates WebRTC engine with OnDataChannel callback
3. Host creates and sends SDP offer via signaling
4. ICE negotiation completes, data channels open
5. Auth channel: client sends password, host verifies
6. Terminal channel: host spawns PTY, pipes I/O
7. Screen channel: host starts screen capture on request
8. On disconnect: cleanup PTY, screen streamer, WebRTC engine

Session state:
- Created (room assigned, WebRTC engine created)
- Authenticated (password verified)
- Active (PTY running, screen streaming)
- Closed (cleanup complete)
```

### Key Code Paths

- `internal/host/daemon.go` → `handleConnectRequest()` creates engine + offer
- `internal/host/daemon.go` → `onDataChannel()` dispatches by label
- `internal/host/daemon.go` → `handleAuthChannel()` validates password
- `internal/host/daemon.go` → `handleTerminalChannel()` spawns PTY, pipes I/O
- `internal/host/daemon.go` → `handleScreenChannel()` manages screen streamer

---

## Appendix: Data Channel Negotiation Detail

### How Data Channels Are Established

The WebRTC protocol supports two modes for data channel negotiation:

1. **In-band (default, `negotiated: false`):** The channel is announced via SDP. The side that calls `createDataChannel()` triggers an `OnDataChannel` event on the remote peer.
2. **Out-of-band (`negotiated: true`):** Both sides create the channel independently with the same ID. No SDP signaling needed.

**remotty uses in-band negotiation** (`negotiated: false`, `ordered: true`).

**Client creates the channels** (terminal, auth). The host receives them via `OnDataChannel`.

This means:
- **For iOS:** The iOS app must call `peerConnection.createDataChannel()` for each channel label AFTER the peer connection is established (or at least after `setLocalDescription` for the answer).
- **For the Go client:** `engine.CreateDataChannel("terminal")` does this.
- **For the host (Go):** It receives channels via `pc.OnDataChannel()` which fires when the client creates them.

### Channel IDs (for reference)

When using in-band negotiation, channel IDs are auto-assigned. The iOS code currently defines specific channel IDs but these are effectively ignored in `negotiated: false` mode:

| Channel | Auto-assigned ID (typical) |
|---------|---------------------------|
| `terminal` | 0 |
| `screen` | 1 |
| `auth` | 2 |
| `file` | 3 |

---

## iOS App Integration Notes

### Current State and Known Issues

The iOS app (`ios/remotty/`) has a `WebRTCService.swift` that implements most of the signaling and WebRTC flow, but several issues need to be resolved:

#### Issue 1: Signaling `connect` message not sent on WebSocket open

**Location:** `WebRTCService.swift`

**Problem:** The `connect` message is only sent from `initializeWebRTCIfNeeded()`, which is called when `room_ready` is received. But the server only sends `room_ready` in response to a `connect` message. This creates a circular dependency — the client never sends `connect`, so it never receives `room_ready`.

**Fix:** Call `sendConnectRequest()` immediately after the WebSocket opens (in the `URLSessionWebSocketDelegate` `didOpenWithProtocol` callback), before waiting for any server messages.

```swift
// In urlSession(_:webSocketTask:didOpenWithProtocol:):
func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, 
                didOpenWithProtocol protocol: String?) {
    webSocketConnected = true
    sendConnectRequest()  // <-- ADD THIS
    receiveWebSocketMessage()
}
```

#### Issue 2: `setupDataChannels()` never called

**Location:** `WebRTCService.swift` line 506

**Problem:** The `setupDataChannels()` method creates the data channels but is never invoked. The iOS app relies on receiving channels via `peerConnection(_:didOpen:)`, but the Go host does not create data channels — the client is supposed to create them.

**Fix:** Call `setupDataChannels()` after setting the local description (after creating the answer):

```swift
// Inside handleOffer's setLocalDescription completion:
self.peerConnection?.setLocalDescription(answer) { error in
    ...
    self.setupDataChannels()  // <-- ADD THIS
    ...
}
```

#### Issue 3: `list_hosts` not implemented for actual host discovery

**Location:** `ConnectionView.swift`

**Problem:** The connection view uses mock data instead of actually connecting to the signaling server and calling `list_hosts`.

**Fix:** Implement a WebSocket connection that:
1. Connects to the signaling server
2. Sends `list_hosts`
3. Receives `host_list` response
4. Populates the `app.hosts` array

#### Issue 4: Hardcoded channel IDs with `isNegotiated = false`

**Location:** `WebRTCService.swift` lines 507-512

**Problem:** The `setupDataChannels()` method sets `channelId` but uses `isNegotiated = false`. With in-band negotiation, `channelId` is ignored (auto-assigned). If the channel ID matters, switch to `isNegotiated = true` and ensure both sides use matching IDs.

**Recommended:** Keep `isNegotiated = false` and remove the explicit `channelId` (it's unused anyway):

```swift
let config = RTCDataChannelConfiguration()
config.isOrdered = true
config.isNegotiated = false
// Remove: config.channelId = channelId(for: label)
```

#### Issue 5: Host list should use `list_hosts` protocol

**Location:** `remottyApp.swift` → `AppState`

**Problem:** The app has no mechanism to fetch the host list from the signaling server.

**Fix:** Add a method to `WebRTCService` (or a separate service) that:
1. Opens a short-lived WebSocket
2. Sends `{ "type": "list_hosts" }`
3. Receives `{ "type": "host_list", "payload": { "hosts": [...] } }`
4. Parses into `[HostInfo]` and updates `app.hosts`

### Correct iOS Connection Sequence

```
1. User enters signalURL and hostID
2. WebSocket connects to signalURL/ws
3. IMMEDIATELY send: { "type": "connect", "payload": { "host_id": hostID } }
4. Wait for server message
5. On "room_ready": create RTCPeerConnectionFactory + RTCPeerConnection
6. Wait for "offer" from server
7. On "offer": setRemoteDescription → createAnswer → setLocalDescription
8. After setLocalDescription: create data channels (terminal, screen, auth, file)
9. Send answer via signaling
10. Exchange ICE candidates (trickle)
11. On ICE connected: send auth + resize
12. Terminal output starts flowing
```

---

## References

| File | Description |
|------|-------------|
| `internal/protocol/message.go` | All message types, envelope, and payload structs |
| `internal/webrtc/engine.go` | WebRTC engine: create offer/answer, ICE, data channels |
| `internal/signal/server.go` | Signaling server: peer management, room creation, relay |
| `internal/host/daemon.go` | Host daemon: registration, session management, PTY/screen |
| `internal/client/client.go` | CLI client: connect, WebRTC setup, terminal I/O |
| `ios/remotty/WebRTCService.swift` | iOS WebRTC + signaling service |
| `web/src/lib/protocol.ts` | TypeScript protocol definitions |
| `web/src/lib/signaling.ts` | TypeScript signaling client |
| `web/src/hooks/useWebRTC.ts` | React WebRTC hook |
