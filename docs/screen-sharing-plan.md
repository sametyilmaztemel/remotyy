# remotty Screen Sharing + iOS App — Development Plan

## Overview

Two features that must work together:
1. **Screen Sharing** — macOS ekranını remote olarak görüntüleme ve kontrol
2. **iOS App** — Native iOS uygulaması ile ekran görüntüleme + touch kontrol

## Architecture

```
┌─────────────────────────┐       WebRTC P2P        ┌──────────────────────┐
│  macOS Host              │                         │  iOS Client           │
│                           │   Video Track (VP8)     │                       │
│  CGDisplay → Capture ────┼─────────────────────────▶│  → Render → UIView    │
│  CGEvent ← Inject  ◄────┼──────────────────────────│  ← Touch → UITouch    │
│                           │   Data Channel (events) │                       │
│  Mouse/Keyboard ←───────┼──────────────────────────│  → Gestures           │
│                           │                         │                       │
│  → 30fps JPEG capture    │                         │  → 60fps display      │
│  → CGEventPost() input   │                         │  → HID touch events   │
└─────────────────────────┘                         └──────────────────────┘
```

## Component Breakdown

### A. macOS Screen Capture (`internal/screen/`)

| Module | Görev | API |
|--------|-------|-----|
| `capture_darwin.go` | Ekran yakalama | CGDisplayCreateImage (CGO) |
| `encoder.go` | JPEG sıkıştırma | turbojpeg / stdlib image/jpeg |
| `stream.go` | WebRTC video track yönetimi | pion/webrtc TrackLocalWriter |
| `input_darwin.go` | Klavye/fare enjeksiyonu | CGEventCreate, CGEventPost |
| `input.go` | Event protokolü | MouseMove, MouseClick, KeyPress |

### B. iOS App (`ios/remotty/`)

| Modül | Görev |
|-------|-------|
| `ScreenView.swift` | Ekran görüntüleme (UIImageView + zoom) |
| `WebRTCService.swift` | WebRTC bağlantı yönetimi |
| `TouchHandler.swift` | Touch → koordinat dönüşümü |
| `TerminalView.swift` | Terminal (mevcut, iyileştirilecek) |
| `ConnectionView.swift` | Bağlantı yönetimi (mevcut) |

### C. Protocol (`internal/protocol/`)

| Mesaj | Tip | Açıklama |
|-------|-----|----------|
| `MsgScreenStart` | Client → Host | Ekran paylaşımını başlat |
| `MsgScreenStop` | Client → Host | Ekran paylaşımını durdur |
| `MsgScreenFrame` | Host → Client | JPEG frame (data channel) |
| `MsgScreenResize` | Host → Client | Ekran boyutu değişikliği |
| `MsgMouseMove` | Client → Host | Fare hareketi (x, y) |
| `MsgMouseClick` | Client → Host | Fare tıklama (button, x, y) |
| `MsgMouseScroll` | Client → Host | Scroll olayı |
| `MsgKeyPress` | Client → Host | Tuş basma (keyCode) |
| `MsgKeyRelease` | Client → Host | Tuş bırakma |

## Development Phases

### Phase 1: Capture & Display (Gün 1-2)
```
macOS:  CGDisplay → JPEG → DataChannel
iOS:    DataChannel → UIImageView → Display
```
- [x] Protocol mesajları (MsgScreenFrame, MsgScreenStart/Stop)
- [ ] macOS: CGDisplayCreateImage ile screenshot
- [ ] macOS: JPEG encode
- [ ] macOS: DataChannel üzerinden frame gönderme
- [ ] iOS: UIImageView ile frame alma ve gösterme
- [ ] iOS: Zoom ve pan jestleri

### Phase 2: Input Forwarding (Gün 3-4)
```
iOS:    Touch → (x, y) → DataChannel
macOS:  DataChannel → CGEventPost → macOS
```
- [x] Protocol mesajları (MsgMouseMove, MsgMouseClick, MsgKeyPress)
- [ ] iOS: UITouch → koordinat dönüşümü
- [ ] iOS: Klavye input capture
- [ ] macOS: CGEventCreate ile mouse hareket
- [ ] macOS: CGEventCreate ile tıklama
- [ ] macOS: CGEventCreate ile klavye

### Phase 3: Performance (Gün 5)
- [ ] Değişen bölgeleri tespit (dirty rect)
- [ ] Adaptive FPS (30fps → 15fps → 5fps)
- [ ] JPEG quality ayarlama (network durumuna göre)
- [ ] VP8 hardware encoding (opsiyonel)

### Phase 4: iOS App Completion (Gün 6-7)
- [ ] Xcode project → build → deploy
- [ ] WebRTC framework entegrasyonu
- [ ] Connection UI polish
- [ ] Screen view pinch-to-zoom
- [ ] Background session handling

## Teknik Detaylar

### macOS Screen Capture (CGO)

```go
// capture_darwin.go
/*
#include <CoreGraphics/CoreGraphics.h>
*/
import "C"

func CaptureDisplay(displayID int) (*image.RGBA, error) {
    imageRef := C.CGDisplayCreateImage(C.uint32_t(displayID))
    // ... convert to *image.RGBA
}
```

### JPEG Encoding

```go
func EncodeJPEG(img *image.RGBA, quality int) ([]byte, error) {
    var buf bytes.Buffer
    jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
    return buf.Bytes(), nil
}
```

### CGEvent Injection

```go
func SendMouseMove(x, y int) error {
    event := C.CGEventCreateMouseEvent(
        nil, C.kCGEventMouseMoved,
        C.CGPointMake(C.CGFloat(x), C.CGFloat(y)),
        C.kCGMouseButtonLeft,
    )
    C.CGEventPost(C.kCGHIDEventTap, event)
    C.CFRelease(C.CFTypeRef(event))
    return nil
}
```

### WebRTC Video Track (pion)

```go
// Alternative: use VP8 video track instead of data channel
track, _ := webrtc.NewTrackLocalStaticSample(
    webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
    "screen", "screen-1",
)
// Write samples to track
track.WriteSample(media.Sample{Data: jpegBytes, Duration: 33 * time.Millisecond})
```

### iOS WebRTC Setup

```swift
// GoogleWebRTC framework
let factory = RTCPeerConnectionFactory()
let config = RTCConfiguration()
config.iceServers = [RTCIceServer(urlStrings: ["stun:stun.l.google.com:19302"])]

let pc = factory.peerConnection(with: config, constraints: constraints, delegate: self)
```

## Data Flow (Full Cycle)

```
[User moves finger on iOS screen]
  → UITouch → (x, y, width, height)
  → remap to macOS coordinates (scaling factor)
  → SendMouseMove {x, y}
  → WebRTC data channel
  → Host receives MsgMouseMove
  → CGEventPost(kCGHIDEventTap, mouseEvent)
  → macOS cursor moves
  → Next screen capture includes new cursor position
  → JPEG frame sent back to iOS
  → Display updated
```

## State Machine

```
IDLE → [Start Screen Share] → CAPTURING
CAPTURING → [Stop Screen Share] → IDLE
CAPTURING → [Client Disconnect] → IDLE
CAPTURING → [Error] → ERROR → [Auto-retry] → CAPTURING
```

## Error Handling

| Hata | Davranış |
|------|----------|
| Screen capture permission denied | Show alert → guide user to grant permission |
| WebRTC connection lost | Auto-reconnect with backoff |
| Low FPS (<5) | Auto-reduce quality → notify user |
| CGEvent permission denied | Show alert → guide to Accessibility permission |

## Permission Requirements

### macOS (Host)
- **Screen Recording**: `ScreenCapture` entitlement + user permission
- **Accessibility**: CGEvent injection requires accessibility permission
- **Camera/Microphone**: Not needed (sadece ekran)

### iOS (Client)
- **Camera**: QR scanning için (isteğe bağlı)
- **Network**: WiFi/cellular

## Testing Strategy

1. **Local test**: macOS host + web client (same machine)
2. **LAN test**: macOS host + iOS client (same WiFi)
3. **Internet test**: macOS host + iOS client (CF Tunnel)
4. **Edge cases**: Low bandwidth, high latency, display sleep

## Files to Create/Modify

### New files:
```
internal/screen/capture_darwin.go    — CGDisplay capture (CGO)
internal/screen/encoder.go           — JPEG encode
internal/screen/input_darwin.go      — CGEvent injection (CGO)
internal/screen/stream.go            — WebRTC video track management
ios/remotty/ScreenView.swift          — iOS screen display
ios/remotty/WebRTCService.swift       — iOS WebRTC
ios/remotty/TouchHandler.swift        — iOS touch → coordinates
ios/remotty/Assets.xcassets/          — iOS app icons
ios/remotty/Info.plist                — iOS app config
remotty-macOS/ScreenCaptureManager.swift — macOS native capture (alt)
```

### Modified files:
```
internal/protocol/message.go         — Screen mesaj tipleri
internal/host/daemon.go              — Screen channel handler
web/src/components/ScreenViewer.tsx   — Web screen viewer update
```
