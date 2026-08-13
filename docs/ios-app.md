# remotty iOS App — Architecture

## Overview

Native iOS client for remotty, written in SwiftUI. Connects to remotty signaling servers,
discovers hosts, and establishes WebRTC connections for terminal and screen access.

## Architecture

```
┌─────────────────────────────────────────────────┐
│  iOS App (SwiftUI)                                │
│                                                    │
│  ┌─────────────────┐  ┌──────────────────┐        │
│  │ ConnectionView   │  │ TerminalView      │        │
│  │ - Signal URL     │  │ - Terminal WebView│        │
│  │ - Master PW      │  │ - Input Bar       │        │
│  └────────┬─────────┘  └────────┬─────────┘        │
│           │                     │                   │
│  ┌────────▼─────────────────────▼─────────┐        │
│  │  WebRTCService                           │        │
│  │  - Signaling (WebSocket)                │        │
│  │  - PeerConnection                       │        │
│  │  - Data Channels                        │        │
│  └─────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────┘
```

## Components

### `remottyApp.swift` — App Entry
- SwiftUI `@main` entry point
- `AppState` ObservableObject for global state
- Connection status tracking

### `ContentView.swift` — Main View
- Host list (or empty state if no hosts)
- Connection status indicator
- Navigation to TerminalView
- Settings button → ConnectionView sheet

### `ConnectionView.swift` — Connection Setup
- Signaling server URL input
- Master password (SecureField)
- Discovery of available hosts
- Saves to AppState

### `TerminalView.swift` — Terminal Emulator
- WebKit-based terminal view
- Input bar with command submission
- Connection status indicator
- TODO: Replace WebKit with native terminal rendering

## WebRTC Integration

The iOS app needs Google WebRTC framework for native WebRTC support:

```swift
// In Package.swift or via CocoaPods
pod 'GoogleWebRTC'
```

Key implementation:
```swift
class WebRTCService: ObservableObject {
    private let factory = RTCPeerConnectionFactory()
    private var pc: RTCPeerConnection?
    
    func connect(to signalURL: String, hostID: String) { ... }
    func sendTerminalInput(_ data: String) { ... }
}
```

## Setup

1. Open `ios/remotty.xcodeproj` in Xcode 15+
2. Install dependencies: `pod install`
3. Set signing team
4. Build to simulator or device

## Requirements

- iOS 17.0+
- Xcode 15.0+
- Swift 5.9+
