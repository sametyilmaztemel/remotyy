# remotty: Feature Gap Analysis & Prioritized Roadmap

> Based on comprehensive analysis of Macky (macky.dev) vs current remotty state
> Generated: 2026-06-04

---

## 1. Current Gap Summary

### 1.1 What Macky Has That remotty Needs

| Feature | Macky | remotty | Priority |
|---------|-------|---------|----------|
| iOS native client (SwiftUI) | ✅ Complete | 🚧 Planned | **P0** |
| Polished macOS menu bar app | ✅ Complete | 🟡 Basic (functional but sparse) | **P1** |
| One-click connection UX | ✅ Excellent | 🟡 Needs polish | **P1** |
| Video demo / marketing site | ✅ Complete | ❌ Not built | P2 |
| Multi-language support | ✅ 6 languages | ❌ Not planned | P3 |
| Background connect | ✅ Pro feature | 🟡 Partial (daemon runs) | **P1** |
| Access Mac with lid closed | ✅ Pro feature | ❌ Not built | **P1** |

### 1.2 What remotty Has That Macky Doesn't

| Feature | remotty | Macky | Status |
|---------|---------|-------|--------|
| Linux host support | ✅ | ❌ | Complete |
| Windows host support | ✅ (planned) | ❌ | 🚧 |
| Web client (xterm.js) | ✅ | ❌ | Complete |
| CLI client | ✅ | ❌ | Complete |
| TUI client (Bubble Tea) | ✅ | ❌ | Complete |
| Self-hosted signaling | ✅ | ❌ | Complete |
| File transfer protocol | ✅ | ❌ | Protocol complete |
| QR pairing | ✅ | ❌ (requires account) | Complete |
| Session recording | ✅ | ❌ | Complete |
| Clipboard sync protocol | ✅ | ❌ | Protocol defined |
| Audit logging | ✅ | ❌ | Complete |
| JavaScript-free web client | ✅ (TUI) | ❌ | Complete |

### 1.3 What Neither Has (Opportunity Space)

- **Port forwarding / TCP tunnel** (high developer demand)
- **Bonjour/mDNS local discovery**
- **Multi-session / pair programming**
- **Wake-on-LAN**
- **Read-only mode**
- **Time-bound access tokens**
- **Android client**
- **End-to-end encrypted persistent storage**

---

## 2. Prioritized Roadmap

### Phase 0: "Production Polish" (Weeks 1-2)
**Goal:** remotty reaches Macky's polish level on existing features.

| # | Task | Area | Effort | Impact | Dependencies |
|---|------|------|--------|--------|-------------|
| **P0.1** | **iOS App MVP** — WebRTC connection + terminal + screen view + touch input | iOS | 2 weeks | 🔥 Critical | `internal/screen/capture_darwin.go` done |
| **P0.2** | **Screen sharing stabilization** — Wire up JPEG frames over WebRTC data channel end-to-end | Backend/iOS | 1 week | 🔥 Critical | WebRTC engine |
| **P0.3** | **Handle 5+ client connections reliability** — Auto-reconnect, handle disconnects gracefully | Core | 3 days | 🔥 Critical | None |
| **P0.4** | **Fix macOS menu bar app** — Proper launchd service, crash recovery, status icon polish | macOS | 3 days | 🔥 Critical | HostManager exists |

**Acceptance criteria:** A user can install the macOS app, start the host, connect from web/CLI/iOS, see the terminal, and share their screen with sub-200ms latency.

---

### Phase 1: "Macky Parity" (Weeks 3-4)
**Goal:** remotty matches or exceeds every Macky feature.

| # | Task | Area | Effort | Impact | Notes |
|---|------|------|--------|--------|-------|
| **1.1** | **iOS app: terminal + auth + connection UI** | iOS | 1 week | 🔥 Critical | Must have full terminal + master password auth |
| **1.2** | **iOS app: screen viewer with pinch-to-zoom** | iOS | 1 week | 🔥 Critical | Match Macky's viewing experience |
| **1.3** | **iOS app: touch injection** — Tap → mouse click, drag → mouse move | iOS | 3 days | 🔥 Critical | CGEvent injection already on host |
| **1.4** | **macOS menu bar polish** — Show connection status, session count, quick-toggle screen share | macOS | 2 days | 🔥 Critical | |
| **1.5** | **Launch at login + auto-start host** | macOS | 1 day | 🔥 Critical | Already partially implemented |
| **1.6** | **Mac lid-closed mode** — `caffeinate` integration, power adapter check | macOS | 2 days | 🔥 Critical | Macky's Pro feature |
| **1.7** | **Background connect** — Allow connecting even when host app is backgrounded | macOS/iOS | 2 days | 🔥 Critical | Macky's Pro feature |
| **1.8** | **Device approval UI** — Show incoming connection request, approve/deny from menu bar | macOS | 2 days | 🔥 Critical | Already in protocol |
| **1.9** | **Connection logs** — 30-day session log viewer (web + macOS) | Web/macOS | 2 days | High | Macky Pro feature |
| **1.10** | **Basic plan session time limits** — Implement 5-min limit for unlicensed users (if wanted) | Core | 1 day | Medium | Only if monetizing |

**Acceptance criteria:** All Macky features are available in remotty with comparable or better UX. Screen sharing works at 15+ FPS on local WiFi.

---

### Phase 2: "Differentiation" (Weeks 5-7)
**Goal:** remotty has features Macky cannot match.

| # | Task | Area | Effort | Impact | Notes |
|---|------|------|--------|--------|-------|
| **2.1** | **File transfer (end-to-end)** — Wire up internal/transfer over WebRTC data channel | Core | 1 week | 🔥🔥 Game-changer | Macky can't do this |
| **2.2** | **File transfer UI** — Web drag-drop zone, CLI `remotty cp`, iOS file picker | All clients | 1 week | 🔥🔥 Game-changer | |
| **2.3** | **Clipboard sync (bidirectional)** — Wire up clipboard protocol + macOS pasteboard watching | Core | 3 days | 🔥🔥 Game-changer | Macky can't do this |
| **2.4** | **Clipboard sync UI** — iOS pasteboard integration, web clipboard API | iOS/Web | 2 days | High | |
| **2.5** | **Port forwarding / TCP tunnel** — `remotty port-forward 3000` → localhost accessible remotely | Core | 2 weeks | 🔥🔥🔥 **Killer** | Macky can't do this |
| **2.6** | **Port forwarding UI** — Web tunnel viewer, CLI tunnel manager | Web/CLI | 1 week | 🔥🔥🔥 **Killer** | |
| **2.7** | **TURN server fallback** — coturn Docker setup, auto-fallback in WebRTC | Core | 2 days | 🔥 Critical | Ensures connectivity everywhere |
| **2.8** | **Bonjour/mDNS local discovery** — Auto-detect hosts on same WiFi | Core/macOS | 3 days | 🔥 High | Zero-config LAN |

**Acceptance criteria:** remotty has at least 3 features (file transfer, clipboard sync, port forwarding) that Macky simply cannot do.

---

### Phase 3: "Developer Experience" (Weeks 8-10)
**Goal:** remotty becomes the go-to remote access tool for developers.

| # | Task | Area | Effort | Impact | Notes |
|---|------|------|--------|--------|-------|
| **3.1** | **Session recording & replay** — Web asciinema player, CLI `remotty replay` | Web/Core | 1 week | 🔥 High | recorder.go already exists |
| **3.2** | **Multi-session / tmux integration** — Auto-tmux, reconnect to running sessions | Core | 1 week | 🔥 High | For pair programming |
| **3.3** | **Multi-client (same session)** — Two people can view/interact with same terminal | Core | 1 week | 🔥 High | Pair debugging |
| **3.4** | **Session management** — `remotty sessions` list, attach, detach, kill | CLI/TUI | 3 days | High | |
| **3.5** | **Wake-on-LAN** — Magic packet sending + config for sleeping machines | Core | 3 days | 🔥 High | |
| **3.6** | **Read-only mode** — Client flag `--read-only`, screen-only mode | Core | 2 days | Medium | Support/debug use case |
| **3.7** | **Time-bound access tokens** — `remotty token create --duration 30m` | Core | 3 days | High | Share access securely |
| **3.8** | **Approval workflow** — Push notification on connect request, allow/deny with time limit | iOS/macOS | 1 week | 🔥 High | Better than Macky's static allow list |

**Acceptance criteria:** A developer can use remotty as their daily driver for remote work — access files, forward ports, record sessions, pair program, manage servers.

---

### Phase 4: "Platform Expansion" (Weeks 11-14)
**Goal:** remotty runs everywhere Macky (and its competitors) don't.

| # | Task | Area | Effort | Impact | Notes |
|---|------|------|--------|--------|-------|
| **4.1** | **Android client** — Kotlin + WebRTC | Android | 3 weeks | 🔥🔥 High | Macky is iOS-only |
| **4.2** | **Windows host daemon** — Cross-compile pion/webrtc, Windows PTY | Windows | 2 weeks | 🔥🔥 High | Use ConPTY or winpty |
| **4.3** | **Audio support** — Pipe Mac audio to remote client (WebRTC audio track) | Core | 2 weeks | Medium | Useful for media monitoring |
| **4.4** | **Marketing site** — Single-page site with hero, comparison table, download links | Web | 1 week | 🔥 High | Copy Macky's effective structure |
| **4.5** | **Multi-language support** — i18n for web + marketing site | Web | 1 week | Medium | Reach global audience |
| **4.6** | **Managed signaling service** — Free hosted signaling for users who don't want self-host | Ops | 2 weeks | High | Lowers barrier to entry |

**Acceptance criteria:** remotty runs on macOS, Linux, Windows (hosts) and iOS, Android, Web, CLI, TUI (clients) — the broadest platform support of any remote access tool.

---

## 3. Competitive Feature Matrix

| Feature | Macky (Free) | Macky ($29) | remotty Now | remotty P0 | remotty P1 | remotty P2 | remotty P3+ |
|---------|:-----------:|:----------:|:----------:|:----------:|:----------:|:----------:|:----------:|
| **Terminal** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Screen sharing** | ✅ | ✅ | 🚧 | ✅ | ✅ | ✅ | ✅ |
| **macOS host** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **iOS client** | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ |
| **Cross-platform host** | ❌ | ❌ | 🟡 (Linux) | 🟡 | 🟡 | 🟡 | ✅ + Win |
| **Web client** | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **CLI client** | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Self-host signaling** | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **File transfer** | ❌ | ❌ | 🟡 (proto) | ❌ | ❌ | ✅ | ✅ |
| **Clipboard sync** | ❌ | ❌ | 🟡 (proto) | ❌ | ❌ | ✅ | ✅ |
| **Port forwarding** | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| **Session recording** | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **QR pairing** | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Multi-session** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| **Bonjour discovery** | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| **Wake-on-LAN** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| **Lid-closed mode** | ❌ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ |
| **Background connect** | ❌ | ✅ | 🟡 | ✅ | ✅ | ✅ | ✅ |
| **Connection logs** | ❌ | ✅ (30d) | 🟡 (audit) | 🟡 | ✅ | ✅ | ✅ |
| **Android client** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| **Price** | $0 | $29/life | **$0 MIT** | **$0 MIT** | **$0 MIT** | **$0 MIT** | **$0 MIT** |

---

## 4. Critical Dependencies & Risk Assessment

### 4.1 Current Weaknesses to Address
| Risk | Severity | Mitigation |
|------|----------|------------|
| No iOS app at all | 🔴 BLOCKER | P0.1 — iOS app is the #1 priority |
| Screen sharing not wired end-to-end | 🔴 BLOCKER | P0.2 — Must work before Macky comparison |
| macOS app is bare-bones | 🟡 Medium | P0.4 + P1.4 — Polish with status, settings, auto-start |
| No real-world testing | 🟡 Medium | Dogfooding — use remotty to access own machines daily |
| No CI/CD pipeline visible | 🟡 Medium | Set up GitHub Actions to build/test all targets |

### 4.2 Technical Risks
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| iOS WebRTC framework complexity | Medium | High | Use GoogleWebRTC pod, test early |
| macOS 15+ API changes (CGDisplayCreateImage obsoleted) | High | Medium | Already has fallback via dlsym |
| CGO cross-compilation for iOS | Medium | High | WebRTC is Objective-C, use Xcode build |
| Screen sharing performance on cellular | Medium | Medium | Adaptive quality already built in stream.go |

---

## 5. Macky's Key UX Patterns to Replicate

1. **Hero section with download buttons** — "Connect to YOUR MACHINE from ANYWHERE"
2. **Feature mockups** — Show the product working, not describe it
3. **Comparison table** — remotty vs TeamViewer vs Tailscale+SSH vs Macky
4. **Security section** — Numbered security layers with clear explanations
5. **One-click download** — Prominent, repeated download CTAs
6. **SEO blog** — Educational content about remote access
7. **Pricing clarity** — Show what's free, what's paid, one-time price

---

## 6. Recommended First Sprint (Week 1)

Given current state, the highest-impact first week:

**Day 1-2:** iOS app: Create Xcode project, integrate WebRTC, establish signaling connection
**Day 3-4:** iOS app: Terminal view with WebRTC data channel, keyboard toolbar
**Day 5:** Wire screen sharing frames over WebRTC data channel end-to-end
**Day 6:** iOS screen viewer with basic touch injection
**Day 7:** Polish + bug fixes + internal dogfooding

**Deliverable:** Functional iOS app that can connect to remotty host, show terminal, and display screen.
