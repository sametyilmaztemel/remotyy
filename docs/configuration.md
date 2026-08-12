# remotty Configuration Reference

## Config File Locations

remotty looks for config in this order:
1. CLI flag: `--config /path/to/remotty.yaml`
2. `./remotty.yaml`
3. `~/.remotty/remotty.yaml`
4. `/etc/remotty/remotty.yaml`

## Global

| Field | Type | Default | Env | Description |
|-------|------|---------|-----|-------------|
| `global.data_dir` | string | `~/.remotty` | — | Data storage directory |
| `global.config_file` | string | — | — | Explicit config file path |

## Signal Server

| Field | Type | Default | Env | Description |
|-------|------|---------|-----|-------------|
| `signal.host` | string | `0.0.0.0` | — | Listen address |
| `signal.port` | int | `9000` | — | Listen port |
| `signal.auth_token` | string | `""` | `REMOTTY_AUTH_TOKEN` | Auth token for WS connections. Empty = no auth. |
| `signal.rate_limit` | int | `60` | — | Requests per minute per IP. 0 = unlimited. |
| `signal.allowed_origins` | []string | `[]` | — | CORS allowed origins. Empty + DevMode = `*`. |
| `signal.dev_mode` | bool | `false` | — | Enable dev mode (relaxed CORS, verbose logging) |
| `signal.web_dir` | string | `""` | — | Directory to serve web UI from |
| `signal.tls.enabled` | bool | `false` | — | Enable TLS |
| `signal.tls.cert_file` | string | `""` | `REMOTTY_TLS_CERT` | TLS certificate path |
| `signal.tls.key_file` | string | `""` | `REMOTTY_TLS_KEY` | TLS key path |

### Auth Token

When `auth_token` is set, WebSocket clients must provide it via:
- Query parameter: `ws://host:9000/ws?token=YOUR_TOKEN`
- Or Authorization header: `Authorization: Bearer YOUR_TOKEN`

### CORS

In production, set `allowed_origins` to your domain:
```yaml
signal:
  allowed_origins:
    - "https://remotty.example.com"
```

In `dev_mode` with no `allowed_origins`, all origins are allowed (`*`).

## Host Daemon

| Field | Type | Default | Env | Description |
|-------|------|---------|-----|-------------|
| `host.signal_url` | string | — | `REMOTTY_SIGNAL_URL` | Signal server URL (e.g. `ws://localhost:9000`) |
| `host.name` | string | hostname | `REMOTTY_HOST_NAME` | Display name for this host |
| `host.master_password` | string | `""` | `REMOTTY_MASTER_PASSWORD` | Plaintext password (auto-hashed on start) |
| `host.master_hash` | string | `""` | — | Pre-hashed password (bcrypt). Use instead of plaintext. |
| `host.require_auth` | bool | `false` | — | Reject daemon start if no password configured |
| `host.allow_list` | []string | `[]` | — | Allowed client IDs. `["*"]` = all. Empty = all. |
| `host.features` | []string | `["terminal"]` | — | Enabled features: terminal, screen, file, clipboard |
| `host.reconnect_wait` | duration | `5s` | — | Initial reconnect wait (doubles on each failure, max 60s) |
| `host.heartbeat_interval` | duration | `15s` | — | Heartbeat send interval |
| `host.session_timeout` | duration | `30m` | — | Session timeout |
| `host.max_sessions` | int | `10` | — | Max concurrent sessions. 0 = unlimited. |
| `host.device_id` | string | — | `REMOTTY_DEVICE_ID` | Stable device identifier |
| `host.show_qr` | bool | `false` | — | Show QR code with connection info in terminal |

## Client

| Field | Type | Default | Env | Description |
|-------|------|---------|-----|-------------|
| `client.signal_url` | string | — | `REMOTTY_SIGNAL_URL` | Signal server URL |
| `client.host_id` | string | — | — | Target host ID or name |
| `client.master_password` | string | `""` | `REMOTTY_MASTER_PASSWORD` | Password for authentication |
| `client.insecure` | bool | `false` | — | Skip TLS verification |

## WebRTC

| Field | Type | Default | Env | Description |
|-------|------|---------|-----|-------------|
| `webrtc.ice_servers` | []string | `["stun:stun.l.google.com:19302"]` | — | STUN/TURN servers |
| `webrtc.mdns` | bool | `true` | — | Enable mDNS candidate gathering |
| `webrtc.ice_timeout` | int | `10` | — | ICE gathering timeout in seconds |
| `webrtc.max_message_size` | int | `65536` | — | Max data channel message size in bytes |

## Logging

| Field | Type | Default | Env | Description |
|-------|------|---------|-----|-------------|
| `logging.level` | string | `info` | `REMOTTY_LOG_LEVEL` | debug, info, warn, error, trace, fatal, disabled |
| `logging.format` | string | `console` | — | Output format: console, json |
| `logging.file` | string | `""` | `REMOTTY_LOG_FILE` | Log file path. Empty = stderr. |

## Screen Sharing

| Field | Type | Default | Env | Description |
|-------|------|---------|-----|-------------|
| `screen.enabled` | bool | `false` | — | Enable screen sharing |
| `screen.fps` | int | `15` | — | Capture frame rate (1-120) |
| `screen.quality` | int | `60` | — | JPEG quality (1-100) |
| `screen.max_dimension` | int | `1920` | — | Max capture dimension (pixels) |
| `screen.capture_cursor` | bool | `false` | — | Include cursor in capture |

## Example Configuration

```yaml
# remotty.yaml — Production configuration

global:
  data_dir: /var/lib/remotty

signal:
  host: 0.0.0.0
  port: 9000
  auth_token: "a-random-secret-token-here"
  rate_limit: 120
  allowed_origins:
    - "https://remotty.example.com"
  tls:
    enabled: true
    cert_file: /etc/ssl/certs/remotty.pem
    key_file: /etc/ssl/private/remotty.key

host:
  signal_url: wss://remotty.example.com:9000
  name: "my-server"
  master_password: "strong-password-here"
  require_auth: true
  max_sessions: 5
  features:
    - terminal
    - screen
    - file
  reconnect_wait: 5s
  heartbeat_interval: 15s

webrtc:
  ice_servers:
    - "stun:stun.l.google.com:19302"
  ice_timeout: 10

logging:
  level: info
  format: json
  file: /var/log/remotty.log

screen:
  enabled: true
  fps: 15
  quality: 70
  max_dimension: 1920
  capture_cursor: true
```
