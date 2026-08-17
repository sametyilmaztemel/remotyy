# remotty

> **Remote terminal & screen access via WebRTC**  
> Cross-platform host daemon + web/CLI/native clients.  
> Open source, unlimited, free.

remotty gives you secure, encrypted remote access to any machine — your Mac, a Linux server, a Raspberry Pi, or a cloud VM — directly from your browser, terminal, or native app. No open ports, no VPN, no SSH key management.

No session limits, no device limits, no time limits. Everything is free and open source.

```
                    ┌──────────────────┐
                    │  Signaling Server│  (self-hosted or cloud)
                    │  ws://host:9000  │
                    └───────┬──────────┘
                            │ WebSocket (blind relay)
                  ┌─────────┴─────────┐
                  ▼                   ▼
          ┌──────────────┐   ┌──────────────────┐
          │  Host Daemon  │   │  Client(s)        │
          │  (any machine)│   │  Web / CLI / iOS  │
          │  pty → shell  │   │  macOS / TUI      │
          │  WebRTC P2P   │◄──┤  xterm.js/Term    │
          │  DTLS-SRTP    │   │  WebRTC DataChan  │
          └──────────────┘   └──────────────────┘
```

## Features

- **🔒 E2E Encrypted** — WebRTC DTLS-SRTP, end-to-end encrypted tunnel
- **🌐 Cross-platform host** — macOS, Linux (ARM64/AMD64), Windows
- **🖥 Web client** — Terminal in your browser via xterm.js
- **📟 CLI client** — `remotty connect` from any terminal
- **📱 iOS app** — Native SwiftUI client
- **🖥 macOS app** — Menu bar host controller
- **🔑 Dual-layer auth** — Signaling token + Master Password (bcrypt)
- **📋 Device allow list** — Explicitly approve devices
- **🕶 Blind signaling** — Server coordinates handshake only
- **📹 Screen sharing** — macOS + Linux
- **📁 File transfer** — Chunked with SHA256 checksums
- **📝 Session recording** — Full terminal capture & replay
- **📊 REST API** — Health checks, host listing, metrics
- **♾️ Unlimited** — No session/device/time limits
- **🧾 MIT License** — Free forever

## Quick Start

### 1. Signaling Server

```bash
# Clone and build
git clone https://github.com/remotty/remotty.git
cd remotty
go build ./cmd/remotty

# Start signaling server (dev mode)
./remotty signal --dev --port 9000
```

### 2. Host Daemon

```bash
./remotty host --signal ws://your-server:9000 --name "my-machine"
```

With a master password (recommended):

```bash
./remotty host --signal ws://your-server:9000 --name "my-machine" \
  --master-password "your-secret-password"
```

Or use QR pairing (zero-config):

```bash
./remotty host --signal ws://your-server:9000 --qr
# → Scan QR code with phone to connect instantly
```

### 3. Connect

**Web client:**
```bash
cd web && npm install && npm run dev
# → http://localhost:3000
```

**CLI client:**
```bash
# List available hosts
./remotty connect --signal ws://your-server:9000

# Connect to a host
./remotty connect host-id --signal ws://your-server:9000
```

## Build

```bash
make build                # CLI binary
make build-all-platforms  # Cross-compile (Linux + macOS)
make build-web            # Web client
make build-macos-app      # macOS menu bar app (requires Xcode)
make build-dmg            # macOS .dmg package
```

## Architecture

```
remotty/
├── cmd/remotty/           # CLI (signal | host | connect)
├── internal/
│   ├── auth/              # bcrypt/argon2 password hashing
│   ├── config/            # Viper config
│   ├── host/              # Host daemon
│   ├── client/            # Client library
│   ├── signal/            # WebSocket signaling server
│   ├── webrtc/            # pion/webrtc engine
│   ├── pty/               # PTY session manager
│   ├── screen/            # Screen capture
│   ├── transfer/          # File transfer
│   ├── qr/                # QR pairing
│   ├── protocol/          # Wire protocol
│   └── logging/           # Audit logging
├── web/                   # React + TypeScript client
├── ios/                   # iOS SwiftUI app
├── remotty-macOS/          # macOS menu bar app
├── src-tauri/             # Tauri desktop wrapper
├── tui/                   # Bubble Tea TUI client
├── deploy/                # Docker, systemd, launchd
└── docs/                  # Documentation
```

## Security

| Layer | Mechanism |
|-------|-----------|
| Transport | WebRTC DTLS-SRTP (E2E encrypted) |
| Signaling | Optional bearer token authentication |
| Device auth | Host must explicitly approve each device |
| Terminal auth | Master Password (bcrypt, never leaves host) |
| Data path | Blind signaling — server only coordinates handshake |
| NAT traversal | STUN/ICE — no open ports required |

## Deployment

- **Single machine:** Run all components on one machine
- **VPS/Cloud:** Signaling on a public server, hosts connect from anywhere
- **Local network:** Same-network with auto-discovery
- **Docker:** `docker compose up`
- **systemd:** Production service files included

## Contributing

Contributions welcome! Read [CONTRIBUTING.md](CONTRIBUTING.md) first.

## License

MIT — see [LICENSE](LICENSE)

---

Built with [pion/webrtc](https://github.com/pion/webrtc) + [xterm.js](https://xtermjs.org/) + [Tauri](https://tauri.app/)
