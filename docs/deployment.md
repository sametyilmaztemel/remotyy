# remotty Deployment Guide

## Quick Start

### 1. Signal Server

```bash
# Build
go build -o remotty ./cmd/remotty

# Run with config
./remotty serve --config remotty.yaml
```

Minimal config:
```yaml
signal:
  port: 9000
  auth_token: "change-me-in-production"
  rate_limit: 60
  allowed_origins:
    - "https://your-domain.com"
```

### 2. Host Daemon

```bash
./remotty host --config host.yaml
```

Host config:
```yaml
host:
  signal_url: ws://your-server:9000
  name: "my-server"
  master_password: "a-strong-password"
  require_auth: true
  max_sessions: 5
  features:
    - terminal
    - screen
    - file
```

### 3. Client

```bash
# TUI client
./remotty connect --host my-server --signal ws://your-server:9000

# Web client — serve via signal server or separately
cd web && npm run build
# Place dist/ in signal.web_dir
```

---

## Production Deployment

### Systemd Service (Signal Server)

```ini
# /etc/systemd/system/remotty-signal.service
[Unit]
Description=remotty Signal Server
After=network.target

[Service]
Type=simple
User=remotty
Group=remotty
ExecStart=/usr/local/bin/remotty serve --config /etc/remotty/signal.yaml
Restart=always
RestartSec=5
LimitNOFILE=65535

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/remotty /var/lib/remotty
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```bash
sudo useradd -r -s /bin/false remotty
sudo mkdir -p /etc/remotty /var/log/remotty /var/lib/remotty
sudo chown remotty:remotty /var/log/remotty /var/lib/remotty
sudo systemctl daemon-reload
sudo systemctl enable --now remotty-signal
```

### Systemd Service (Host Daemon)

```ini
# /etc/systemd/system/remotty-host.service
[Unit]
Description=remotty Host Daemon
After=network.target remotty-signal.service

[Service]
Type=simple
User=remotty
Group=remotty
ExecStart=/usr/local/bin/remotty host --config /etc/remotty/host.yaml
Restart=always
RestartSec=5

# PTY access needs devpts
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/var/log/remotty /var/lib/remotty /dev/pts
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### Docker Compose

```yaml
version: "3.8"

services:
  signal:
    image: remotty/remotty:latest
    command: serve --config /etc/remotty.yaml
    ports:
      - "9000:9000"
    volumes:
      - ./remotty.yaml:/etc/remotty.yaml:ro
      - signal-data:/var/lib/remotty
    restart: always
    security_opt:
      - no-new-privileges:true

  host:
    image: remotty/remotty:latest
    command: host --config /etc/host.yaml
    volumes:
      - ./host.yaml:/etc/host.yaml:ro
      - host-data:/var/lib/remotty
    depends_on:
      - signal
    restart: always
    # PTY support
    privileged: false
    devices:
      - /dev/pts:/dev/pts

volumes:
  signal-data:
  host-data:
```

### Reverse Proxy (Nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name remotty.example.com;

    ssl_certificate /etc/ssl/certs/remotty.pem;
    ssl_certificate_key /etc/ssl/private/remotty.key;

    # Web UI
    location / {
        proxy_pass http://127.0.0.1:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket
    location /ws {
        proxy_pass http://127.0.0.1:9000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

### Cloudflare Tunnel

```bash
cloudflared tunnel create remotty
cloudflared tunnel route dns remotty remotty.example.com

# ~/.cloudflared/config.yml
tunnel: remotty
credentials-file: ~/.cloudflared/<TUNNEL_ID>.json

ingress:
  - hostname: remotty.example.com
    service: http://localhost:9000
  - service: http_status:404
```

---

## Security Checklist

- [ ] `auth_token` set to a strong random value
- [ ] `allowed_origins` configured (not `*`)
- [ ] `dev_mode` is `false`
- [ ] TLS enabled (or behind reverse proxy with TLS)
- [ ] `rate_limit` set (60-120 req/min recommended)
- [ ] Host `require_auth` is `true` with strong password
- [ ] `max_sessions` limited per host
- [ ] Firewall: only expose port 443/80 (reverse proxy)
- [ ] Log rotation configured
- [ ] Regular updates applied

## Monitoring

```bash
# Health check
curl http://localhost:9000/health

# List connected hosts
curl http://localhost:9000/api/hosts
```

## Troubleshooting

**WebSocket connection refused:**
- Check auth token matches between client and server
- Verify `allowed_origins` includes client origin
- Check rate limit isn't too low

**Host not appearing in list:**
- Verify `signal_url` is reachable from host
- Check host logs for connection errors
- Ensure heartbeat is working (default 15s interval)

**WebRTC connection fails:**
- Verify STUN/TURN servers are accessible
- Check firewall allows UDP traffic for ICE
- For NAT traversal issues, consider adding TURN server
