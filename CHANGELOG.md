# Changelog

All notable changes to remotty will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Security**: Auth token validation on WebSocket upgrade (query param + Bearer header)
- **Security**: CORS origin restriction (`allowed_origins` config)
- **Security**: Per-IP token bucket rate limiter (configurable `rate_limit`)
- **Security**: Max sessions enforcement on host daemon
- **Security**: WebSocket read deadline with heartbeat timeout (45s)
- **Security**: `require_auth` config to reject daemon start without password
- **Stability**: Config validation (15 rules: port, TLS, timing, screen, WebRTC)
- **Stability**: Exponential reconnect backoff (5s → 60s max)
- **Stability**: Structured error codes (ranges: 1xxx auth, 2xxx connection, 3xxx protocol, 4xxx session, 5xxx internal)
- **Testing**: 200+ Go tests across 13 packages
- **Testing**: 43 web client tests (vitest)
- **CI**: Go 1.23/1.24 matrix, Node 22, `go vet`, golangci-lint, race detector
- **CI**: Cross-platform release pipeline (darwin-arm64, linux-arm64, linux-amd64)
- **Docs**: Configuration reference with all fields, types, defaults
- **Docs**: Deployment guide (systemd, Docker Compose, Nginx, Cloudflare Tunnel)
- **Docker**: Multi-stage Dockerfile with web client build
