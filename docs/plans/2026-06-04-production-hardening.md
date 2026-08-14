# remotty Production Hardening Plan

> **For Hermes ARM:** Execute task-by-task, commit after each phase.

**Goal:** Bring remotty from v0.7.1 to production-ready quality — secure, stable, well-tested, properly configured.

**Architecture:** Signal Server (WebSocket relay) + Host Daemon (remote machine agent) + Client (TUI/Web/Swift). WebRTC P2P for data channels.

---

## Phase 1: Security Hardening (CRITICAL)

### Task 1.1: Auth token validation on signal server
- **Problem:** `SignalConfig.AuthToken` exists but never checked. Anyone can connect.
- **Files:** `internal/signal/server.go` (handleWebSocket)
- **Fix:** If `cfg.AuthToken != ""`, validate `Authorization: Bearer <token>` header on WS upgrade. Reject if missing/invalid.
- **Test:** TestAuthTokenRequired, TestAuthTokenInvalid, TestAuthTokenValid

### Task 1.2: CORS restriction
- **Problem:** `Access-Control-Allow-Origin: *` — any website can connect.
- **Fix:** Add `SignalConfig.AllowedOrigins []string`. If set, validate `Origin` header. If empty+DevMode, allow all. If empty+!DevMode, allow only same-origin.
- **Files:** `internal/signal/server.go` (upgrader, withMiddleware), `internal/config/config.go`
- **Test:** TestCORSAllowOrigin, TestCORSBlockOrigin

### Task 1.3: Rate limiting middleware
- **Problem:** `SignalConfig.RateLimit` exists (default 60) but never enforced.
- **Fix:** Implement per-IP token bucket rate limiter. Apply to WebSocket connections and HTTP endpoints.
- **Files:** `internal/signal/server.go` (withMiddleware), new `internal/signal/ratelimit.go`
- **Test:** TestRateLimitEnforced, TestRateLimitDifferentIPs

### Task 1.4: Host daemon auth bypass check
- **Problem:** `MasterHash == ""` means ANYONE can connect without password. This is fine for local dev but dangerous in prod.
- **Fix:** In `NewDaemon`, if no password/hash set, log a WARNING. Add config option `host.require_auth bool`. If `require_auth=true` and no password → return error.
- **Files:** `internal/host/daemon.go`, `internal/config/config.go`
- **Test:** TestDaemonNoAuthWarning, TestDaemonRequireAuth

### Task 1.5: Max sessions enforcement
- **Problem:** `MaxSessions` config exists (default 0=unlimited in config, 10 in daemon). Daemon sets it but never checks.
- **Fix:** In `handleConnectRequest`, check `len(sessions) >= cfg.MaxSessions`. Reject if at limit.
- **Files:** `internal/host/daemon.go`
- **Test:** TestMaxSessionsEnforced

### Task 1.6: WebSocket read deadline / heartbeat timeout
- **Problem:** No read deadline on WS connections. Dead peers never detected (heartbeat is sent but missed heartbeats aren't detected).
- **Fix:** Set read deadline on each read. On heartbeat, reset deadline. Add `PeerDeadline = 45s` (3x heartbeat interval 15s). If deadline expires, close peer.
- **Files:** `internal/signal/server.go` (readLoop, handleHeartbeat)
- **Test:** TestHeartbeatDeadlineExpires, TestHeartbeatResetsDeadline

---

## Phase 2: Stability & Error Handling

### Task 2.1: Config validation is empty
- **Problem:** `Config.Validate()` is a no-op (empty body with comments).
- **Fix:** Implement real validation:
  - Signal host:port format
  - TLS cert+key both present if TLS enabled
  - ReconnectWait > 0
  - HeartbeatInt > 0
  - SessionTimeout > 0
  - Log level valid value
  - Screen FPS/Quality in range
- **Files:** `internal/config/config.go`
- **Test:** Update `config_test.go` — TestValidateValid, TestValidateInvalid

### Task 2.2: Graceful shutdown improvements
- **Problem:** Daemon.Run() doesn't propagate shutdown to all goroutines cleanly.
- **Fix:** Use errgroup or tracked goroutines. Ensure all PTY sessions, screen streamers, WebRTC engines closed before exit. Add timeout.
- **Files:** `internal/host/daemon.go`
- **Test:** TestDaemonGracefulShutdown (mock WS)

### Task 2.3: Reconnect backoff
- **Problem:** Daemon reconnects with fixed `ReconnectWait` (5s). Should use exponential backoff with jitter.
- **Fix:** Implement `reconnectBackoff(base, max, attempt) → duration`. Apply in `Run()` loop.
- **Files:** `internal/host/daemon.go`
- **Test:** TestReconnectBackoff

### Task 2.4: Error codes standardization
- **Problem:** ErrorPayload.Code is always 0. Messages are inconsistent strings.
- **Fix:** Define error code constants. Use structured codes: 1000-1999 auth, 2000-2999 connection, 3000-3999 protocol.
- **Files:** New `internal/protocol/errors.go`
- **Test:** TestErrorCodeConstants

---

## Phase 3: Host Daemon Test Coverage

### Task 3.1: Mock WebSocket infrastructure
- Create `internal/host/testhelpers_test.go` with mock WS server (similar to signal test pattern).
- Allows testing daemon without real network.

### Task 3.2: Daemon registration lifecycle test
- Test: NewDaemon → Run → connect → register → receive peer ID → OnRegistered callback.

### Task 3.3: Daemon session management test
- Test: handleConnectRequest creates session, handlePeerDisconnect cleans up.
- Test: max sessions rejection.
- Test: allow list enforcement.

### Task 3.4: Auth channel test
- Test: correct password → auth_ok, wrong password → auth_fail, no password required → auth_ok.

---

## Phase 4: Signal Server Test Coverage

### Task 4.1: Concurrent registration stress test
- Multiple goroutines register simultaneously. Verify no race conditions.

### Task 4.2: Room lifecycle test
- Create room → relay messages → peer disconnect → room cleanup → other peer notified.

### Task 4.3: Auth token integration test
- Server with auth token → valid connection works, invalid rejected.

---

## Phase 5: CI/CD & Tooling

### Task 5.1: golangci-lint in CI
- Add lint step to CI: `golangci-lint run ./...`
- Fail on warnings.

### Task 5.2: Go vet + race detector in CI
- Add `-race` flag to test step.
- Add `go vet ./...` step.

### Task 5.3: Release binary verification
- Ensure GoReleaser config produces ARM64 + AMD64 binaries.
- Add checksums and signing.

### Task 5.4: Web client prod build verification
- Ensure `npm run build` produces optimized bundle.
- Check bundle size < threshold.

---

## Phase 6: Documentation

### Task 6.1: Config reference
- Document all config fields with types, defaults, env vars.

### Task 6.2: Security hardening guide
- Document: auth token setup, TLS config, CORS, rate limiting, firewall recommendations.

### Task 6.3: Deployment guide
- Systemd unit files, Docker compose, reverse proxy config.

---

## Execution Order

1. Phase 1 (Security) — 6 tasks, highest priority
2. Phase 2 (Stability) — 4 tasks
3. Phase 3 + 4 (Tests) — 7 tasks
4. Phase 5 (CI/CD) — 4 tasks
5. Phase 6 (Docs) — 3 tasks

**Total: 24 tasks, ~15-20 commits**

Start with Phase 1 Task 1.1 (auth token validation).
