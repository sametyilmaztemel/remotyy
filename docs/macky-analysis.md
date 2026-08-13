# Macky Comprehensive Feature Analysis

> Analysis performed: 2026-06-04
> Product: https://macky.dev/
> Company: Velosify Private Limited (India)
> Target audience: Mac users who want to access their computer from iPhone

---

## 1. Product Overview

Macky is a **Mac-to-iPhone remote access tool** that provides full desktop viewing, terminal access, and screen control through an E2E encrypted WebRTC connection. It is a closed-source, commercial product priced at $29 lifetime (with a free tier).

**Core value proposition:** "Connect to YOUR MAC from iPhone. No VPN, no port forwarding, no technical setup."

**Tagline:** App Store simplicity for power-user remote access.

---

## 2. Complete Feature List

### 2.1 Remote Desktop / Screen Sharing
- **Full desktop viewing** — See your Mac's screen live on iPhone
- **Remote control** — Move cursor, click, type on Mac from iPhone
- **Pinch-to-zoom** on iPhone for desktop view
- **Landscape + portrait** support
- **Adaptive quality** — Adjusts to network conditions

### 2.2 Native Terminal (ZSH/BASH)
- Full interactive terminal on iPhone
- Ctrl key in keyboard toolbar
- Resize support
- Paste support

### 2.3 Security Features
| Layer | Feature | Description |
|-------|---------|-------------|
| Transport | **E2E Encrypted WebRTC** | DTLS-SRTP tunneling, data invisible to network |
| Verification | **Dual Layer Identity** | Signaling identity tokens + Master Password |
| Authorization | **Device Allow Listing** | Host must explicitly approve each unique device ID |
| Privacy | **Blind Signaling** | Server coordinates handshake only; terminal data never touches cloud |

### 2.4 Connectivity
- **Zero open ports** — No port forwarding needed
- **Direct P2P** — Sub-50ms typical latency
- **Network switch survival** — Seamless WiFi → cellular transitions
- **STUN-based NAT traversal** — Works on hotel, corporate, coffee shop WiFi
- **Works on cellular** — 4G/5G support

### 2.5 Platform Support
| Component | Support |
|-----------|---------|
| Host (Mac) | **macOS 15+ only** (Sequoia) |
| Client (iPhone/iPad) | **iOS 18+ only** |
| Linux | ❌ None |
| Windows | ❌ None |
| Web client | ❌ None |
| CLI client | ❌ None |

### 2.6 Onboarding & UX
- Install Mac app (DMG download) → Install iOS app (App Store) → Create account → Set Master Password → Connect
- **No router config, no SSH keys, no port forwarding**
- **Menu bar app** on macOS (runs in background)
- **Native iOS app** with SwiftUI

### 2.7 Pricing Model
| Plan | Price | Limits |
|------|-------|--------|
| **Basic** | **$0/forever** | 5-min max session, 1 Mac host, 1 iPhone remote |
| **Pro** | **$29 lifetime** (one-time) | Unlimited sessions, unlimited devices, 30-day connection logs, background connect, access Mac with lid closed |

Key: **No subscription.** One-time payment.

### 2.8 Blog / Educational Content
13 blog posts covering:
- How to access Mac remotely
- How to view Mac screen on iPhone
- How to access files on Mac from iPhone
- How to control Mac from another room
- How to run Claude Code on iPhone
- How to access Mac without SSH
- WebRTC vs SSH comparison
- How to monitor home server from iPhone
- Best ways to code on iPhone 2026
- How to use Git from iPhone
- Remote Desktop for Mac options compared
- How to connect to Mac terminal from iPhone

---

## 3. UX/UI Patterns

### 3.1 Visual Design
- **Dark mode** only — Black background, white text, purple accents (#8b5cf6)
- **Typography** — Large serif headlines (italic) mixed with clean sans-serif body text
- **Spacious layout** — Single-column flow expanding into comparison grids
- **Minimalist** — Very few UI elements, focused messaging

### 3.2 Homepage Structure
1. **Hero** — "Connect to YOUR MAC from iPhone." with two download buttons
2. **Feature showcase** — Three mockups: Demo (browser), Full Remote Desktop, Native Shell
3. **Defense in Depth** — Four security layers with descriptions
4. **Macky vs old way** — Comparison table vs Tailscale + SSH
5. **Your Turn** — Download cards for macOS and iOS
6. **Footer** — Terms, Privacy, Blog, support email

### 3.3 Navigation
- ENGINE (anchor: #engine)
- SECURITY (anchor: #architecture)
- BLOG (/blog)
- PRICING (/en/pricing)
- DOWNLOAD (anchor: #download)
- Language select (6 languages)

### 3.4 Key UX Decisions
- **No signup upfront** — "Download first, ask questions later" approach
- **Comparison-driven** — Shows how Macky is easier than alternatives
- **Security forward** — Security is the second section after features
- **Blog as growth engine** — SEO-optimized educational content
- **One-time payment** — Reduces purchase friction vs subscription
- **Multi-language** — Localized into 6 languages (EN, FR, DE, ES, JA, PT, KO)

---

## 4. Onboarding Flow

```
1. Visit macky.dev
2. Download Mac DMG (or iOS app from App Store)
3. Install Mac app (menu bar app, runs in background)
4. Install iOS app (App Store)
5. Create account (email + password)
6. Sign in on both devices with same account
7. Set Master Password (protects terminal even if account compromised)
8. Open iPhone app → tap Mac from list → enter Master Password
9. Connected!
```

**Total time claimed:** ~2 minutes
**Friction points:** Requires account creation (unlike QR-based pairing)

---

## 5. Architecture Observations (Inferred)

Based on website claims and technical blog posts:

| Component | Macky Implementation |
|-----------|---------------------|
| **Signaling** | Velosify cloud servers (cannot self-host) |
| **NAT traversal** | ICE + STUN (Google STUN servers) |
| **Fallback** | Likely uses TURN relays for strict NAT |
| **Encryption** | DTLS 1.3 + SRTP |
| **Screen capture** | macOS native (likely CGDisplayCreateImage) |
| **Terminal** | macOS PTY (pseudoterminal) attached to WebRTC |
| **Auth** | Account-based + Master Password (bcrypt) |
| **Device ID** | Unique per device, host must approve |
| **File transfer** | ❌ Not supported |
| **Clipboard sync** | ❌ Not supported |
| **Session recording** | ❌ Not supported |

---

## 6. Macky's Differentiators

1. **App Store simplicity** — Install two apps, done. No technical knowledge needed.
2. **Zero configuration** — No router, no VPN, no SSH keys, no port forwarding.
3. **One-time payment** — $29 lifetime, no subscription fatigue.
4. **Blind signaling** — Strong privacy promise, data never touches their servers.
5. **macOS-specific polish** — Deep integration with macOS (menu bar, accessibility APIs).
6. **iOS-first** — Optimized for iPhone experience (touch controls, pinch-to-zoom).
7. **Network resilience** — Survives WiFi → cellular handoff.

---

## 7. Macky's Weaknesses / Gaps

### 7.1 Platform Limitations
| Gap | Impact |
|-----|--------|
| **macOS host only** (15+) | Cannot access Linux servers, Raspberry Pi, Windows, older macOS |
| **iOS client only** (18+) | No Android, web, or desktop client |
| **No web client** | Cannot connect from a browser at a friend's computer or library |
| **No CLI client** | Developers can't script or automate connections |

### 7.2 Missing Features (remotty has or could have)
| Feature | Macky | remotty |
|---------|-------|---------|
| File transfer | ❌ | ✅ (built, needs WebRTC integration) |
| Clipboard sync | ❌ | 🚧 (protocol defined) |
| Port forwarding | ❌ | Planned (killer feature) |
| Session recording | ❌ | ✅ (built) |
| Multi-session | ❌ | Planned |
| Local discovery (Bonjour) | ❌ | Planned |
| Self-hosted signaling | ❌ | ✅ (full support) |
| TURN fallback | Unknown | Planned |
| Wake-on-LAN | ❌ | Planned |
| Audit logging | ❌ | ✅ (built) |
| Read-only mode | ❌ | Planned |
| Time-bound access tokens | ❌ | Planned |

### 7.3 Business Model Risks
- **One company's cloud** — If Velosify shuts down, Macky stops working
- **No self-host** — Users cannot run their own signaling server
- **India jurisdiction** for legal disputes
- **Closed source** — No community contributions, no custom modifications

### 7.4 Technical Constraints
- **Mac must be awake** (basic plan)
- **5-min session limit** on free plan
- **Lid-closed mode** requires Pro ($29)
- **30-day connection log retention** only on Pro

---

## 8. Key Insights for remotty

### 8.1 What Macky Does Right
1. **Simple onboarding** — Install two apps, done. remotty needs to match this simplicity.
2. **Security storytelling** — "Defense in Depth" with 4 numbered layers is effective marketing.
3. **Direct comparison** — Macky vs "old way" comparison table is convincing.
4. **Educational blog** — SEO content drives organic discovery.
5. **One-time pricing** — $29 lifetime is a great psychological price point.
6. **Zero setup** — The core message "no VPN, no port forwarding" resonates with users.

### 8.2 Where remotty Can Win
1. **Self-hosted** — Own your signaling, zero ongoing cost, no vendor lock-in.
2. **Cross-platform host** — macOS + Linux + Windows (Macky is macOS-only).
3. **Cross-platform client** — Web + CLI + TUI + iOS + macOS + Android (Macky is iOS-only).
4. **File transfer** — Macky doesn't have it, remotty does.
5. **Clipboard sync** — Macky doesn't have it.
6. **Port forwarding** — The killer developer feature Macky can't do.
7. **Session recording** — Asciinema-style replay for audit.
8. **QR pairing** — No account needed, just scan and connect.
9. **Open source MIT** — Community trust, customizability.
10. **Unlimited free** — No session limits, no device limits.

### 8.3 Pricing Comparison
| | Macky Basic | Macky Pro | remotty |
|--|-----------|----------|---------|
| Price | $0 | $29 lifetime | **$0 (MIT)** |
| Session limit | 5 min | Unlimited | **Unlimited** |
| Device limit | 1 Mac, 1 iPhone | Unlimited | **Unlimited** |
| Signaling | Velosify cloud | Velosify cloud | **Self-host** |
| License | Proprietary | Proprietary | **MIT open source** |

### 8.4 Critical UX Lessons from Macky
1. **Show, don't tell** — Macky's homepage has video demo and mockup images
2. **Compare yourself** — The Macky vs Tailscale+SSH table is convincing
3. **Lead with security** — Security as a feature, not an afterthought
4. **Price prominently** — $29 lifetime shown on pricing page clearly
5. **Blog for SEO** — Educational content drives organic traffic
6. **Multi-language** — International audience matters
7. **Dark mode** — Professional, developer-friendly aesthetic
