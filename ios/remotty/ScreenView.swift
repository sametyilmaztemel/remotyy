import SwiftUI
import UIKit
import Combine

// MARK: - SwiftUI Screen View

/// Full‑screen remote display that renders JPEG frames from the WebRTC screen
/// data channel, supports pinch‑to‑zoom/pan, and forwards touch events as
/// remote mouse/keyboard input.
struct ScreenView: View {
    let host: HostInfo
    @EnvironmentObject private var app: AppState

    // Local state driven by WebRTC service callbacks
    @State private var fps: Int = 0
    @State private var connectionState: WebRTCState = .disconnected
    @State private var showKeyboard = false

    var body: some View {
        ZStack {
            // UIKit render surface
            ScreenRendererViewRepresentable(
                webRTCService: app.webRTCService,
                fps: $fps,
                connectionState: $connectionState
            )
            .ignoresSafeArea()

            // Overlays
            overlayContent
        }
        .navigationBarHidden(true)
        .statusBar(hidden: true)
        .onAppear {
            connectToHost()
        }
        .onDisappear {
            disconnectFromHost()
        }
        .sheet(isPresented: $showKeyboard) {
            keyboardInputSheet
        }
    }

    // MARK: - Connection

    private func connectToHost() {
        let signalURL = app.signalURL
        app.webRTCService.delegate = nil // will reset in representable
        app.webRTCService.connect(
            signalURL: signalURL,
            hostID: host.id,
            password: nil
        )
    }

    private func disconnectFromHost() {
        app.webRTCService.disconnect()
    }

    // MARK: - Overlays

    @ViewBuilder
    private var overlayContent: some View {
        // Top bar: connection status
        VStack {
            statusBar
            Spacer()
            // Bottom bar: FPS + keyboard button
            bottomBar
        }
    }

    @ViewBuilder
    private var statusBar: some View {
        HStack {
            // Connection indicator
            HStack(spacing: 6) {
                Circle()
                    .fill(statusColor)
                    .frame(width: 8, height: 8)
                Text(statusText)
                    .font(.caption2.monospaced())
                    .foregroundColor(.white)
            }
            .padding(.horizontal, 10)
            .padding(.vertical, 4)
            .background(.ultraThinMaterial, in: Capsule())

            Spacer()

            // Host name
            Text(host.name)
                .font(.caption2.monospaced())
                .foregroundColor(.white.opacity(0.7))
                .padding(.horizontal, 10)
                .padding(.vertical, 4)
                .background(.ultraThinMaterial, in: Capsule())
        }
        .padding(.horizontal)
        .padding(.top, 6)
    }

    @ViewBuilder
    private var bottomBar: some View {
        HStack {
            // FPS counter
            if fps > 0 {
                Text("\(fps) FPS")
                    .font(.caption2.monospaced().weight(.semibold))
                    .foregroundColor(fpsColor)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 3)
                    .background(.ultraThinMaterial, in: Capsule())
            }

            Spacer()

            // Keyboard toggle
            Button(action: { showKeyboard.toggle() }) {
                Image(systemName: showKeyboard ? "keyboard.fill" : "keyboard")
                    .font(.caption)
                    .foregroundColor(.white)
                    .padding(8)
                    .background(.ultraThinMaterial, in: Circle())
            }
        }
        .padding(.horizontal)
        .padding(.bottom, 8)
    }

    // MARK: Keyboard input sheet

    private var keyboardInputSheet: some View {
        VStack(spacing: 0) {
            // Drag handle
            RoundedRectangle(cornerRadius: 2.5)
                .fill(Color.secondary.opacity(0.4))
                .frame(width: 36, height: 5)
                .padding(.top, 8)

            TextField("Type to send keystrokes...", text: .constant(""))
                .textFieldStyle(.plain)
                .font(.title3.monospaced())
                .padding()
                .background(Color(.systemGray6))
                .onChange(of: showKeyboard) { _ in }
                .onSubmit {
                    // Handled via delegation
                }
        }
        .presentationDetents([.height(120)])
        .presentationDragIndicator(.hidden)
    }

    // MARK: - Derived state

    private var statusColor: Color {
        switch connectionState {
        case .disconnected: return .gray
        case .connecting:   return .yellow
        case .connected:    return .green
        case .error:        return .red
        }
    }

    private var statusText: String {
        switch connectionState {
        case .disconnected: return "Disconnected"
        case .connecting:   return "Connecting..."
        case .connected:    return "Connected"
        case .error(let msg): return "Error: \(msg)"
        }
    }

    private var fpsColor: Color {
        if fps >= 25 { return .green }
        if fps >= 10 { return .yellow }
        return .red
    }
}

// MARK: - UIViewRepresentable Bridge

/// Bridges the UIKit `ScreenRendererView` into SwiftUI.
private struct ScreenRendererViewRepresentable: UIViewRepresentable {

    let webRTCService: WebRTCService
    @Binding var fps: Int
    @Binding var connectionState: WebRTCState

    func makeUIView(context: Context) -> ScreenRendererView {
        let view = ScreenRendererView()
        view.webRTCService = webRTCService
        view.touchHandler.webRTCService = webRTCService
        view.touchHandler.remoteScreenSize = webRTCService.remoteScreenSize

        // Wire up frame delivery
        webRTCService.delegate = context.coordinator

        // FPS callback from render view
        view.onFPSUpdate = { [self] newFPS in
            fps = newFPS
        }

        // Observe size changes for coordinate mapping
        view.onLayoutChanged = { [weak view] size in
            view?.touchHandler.viewSize = size
        }

        return view
    }

    func updateUIView(_ uiView: ScreenRendererView, context: Context) {
        uiView.touchHandler.remoteScreenSize = webRTCService.remoteScreenSize
        uiView.touchHandler.viewSize = uiView.bounds.size
    }

    func makeCoordinator() -> Coordinator {
        Coordinator(parent: self)
    }

    // MARK: Coordinator — WebRTC delegate

    final class Coordinator: NSObject, WebRTCServiceDelegate {
        private let parent: ScreenRendererViewRepresentable

        init(parent: ScreenRendererViewRepresentable) {
            self.parent = parent
        }

        func webRTCService(_ service: WebRTCService, didReceiveFrame image: UIImage) {
            // Forwarded directly to the render view via the coordinator's weak reference
            // The render view subscribes to service.currentFrame via Combine or direct observation
            // We handle this by setting up observation in makeUIView
        }

        func webRTCService(_ service: WebRTCService, didChangeState state: WebRTCState) {
            DispatchQueue.main.async { [weak self] in
                self?.parent.connectionState = state
            }
        }

        func webRTCService(_ service: WebRTCService, didReceiveTerminalOutput text: String) {
            // Not relevant for screen view — ignored
        }

        func webRTCService(_ service: WebRTCService, didReceiveError error: Error) {
            DispatchQueue.main.async { [weak self] in
                self?.parent.connectionState = .error(error.localizedDescription)
            }
        }
    }
}

// MARK: - UIKit Render View

/// UIView that displays remote screen frames, handles gestures and touch events,
/// and forwards input to the TouchHandler.
final class ScreenRendererView: UIView {

    // MARK: Dependencies

    weak var webRTCService: WebRTCService? {
        didSet {
            observeFrames()
        }
    }
    let touchHandler = TouchHandler()

    // MARK: Callbacks

    var onFPSUpdate: ((Int) -> Void)?
    var onLayoutChanged: ((CGSize) -> Void)?

    // MARK: UI

    private let imageView = UIImageView()
    private let activityIndicator = UIActivityIndicatorView(style: .large)

    // MARK: Zoom state

    private var currentScale: CGFloat = 1.0 {
        didSet { applyTransform() }
    }
    private var currentOffset: CGPoint = .zero {
        didSet { applyTransform() }
    }
    private let minZoomScale: CGFloat = 1.0
    private let maxZoomScale: CGFloat = 5.0
    private var lastPinchScale: CGFloat = 1.0

    // MARK: FPS tracking

    private var frameCount: Int = 0
    private var lastFPSTime: CFTimeInterval = CACurrentMediaTime()
    private var displayLink: CADisplayLink?

    // MARK: Observation

    private var frameObservation: AnyCancellable?

    // MARK: Init

    override init(frame: CGRect) {
        super.init(frame: frame)
        setup()
    }

    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setup()
    }

    private func setup() {
        backgroundColor = .black

        // Image view
        imageView.contentMode = .scaleAspectFit
        imageView.clipsToBounds = true
        imageView.translatesAutoresizingMaskIntoConstraints = false
        addSubview(imageView)
        NSLayoutConstraint.activate([
            imageView.topAnchor.constraint(equalTo: topAnchor),
            imageView.bottomAnchor.constraint(equalTo: bottomAnchor),
            imageView.leadingAnchor.constraint(equalTo: leadingAnchor),
            imageView.trailingAnchor.constraint(equalTo: trailingAnchor),
        ])

        // Activity indicator (shown while connecting)
        activityIndicator.translatesAutoresizingMaskIntoConstraints = false
        addSubview(activityIndicator)
        NSLayoutConstraint.activate([
            activityIndicator.centerXAnchor.constraint(equalTo: centerXAnchor),
            activityIndicator.centerYAnchor.constraint(equalTo: centerYAnchor),
        ])
        activityIndicator.startAnimating()

        // Gesture recognizers
        let pinch = UIPinchGestureRecognizer(target: self, action: #selector(handlePinch(_:)))
        addGestureRecognizer(pinch)

        let pan = UIPanGestureRecognizer(target: self, action: #selector(handlePan(_:)))
        pan.minimumNumberOfTouches = 2
        pan.maximumNumberOfTouches = 2
        addGestureRecognizer(pan)

        let doubleTap = UITapGestureRecognizer(target: self, action: #selector(handleDoubleTap(_:)))
        doubleTap.numberOfTapsRequired = 2
        addGestureRecognizer(doubleTap)

        // Two-finger tap for right-click
        let twoFingerTap = UITapGestureRecognizer(target: self, action: #selector(handleTwoFingerTap(_:)))
        twoFingerTap.numberOfTouchesRequired = 2
        addGestureRecognizer(twoFingerTap)

        // Touch handling
        isMultipleTouchEnabled = true

        // Hardware keyboard capture
        canBecomeFirstResponder = true

        // Display link for FPS calculation
        displayLink = CADisplayLink(target: self, selector: #selector(displayLinkTick(_:)))
        displayLink?.add(to: .main, forMode: .common)
    }

    deinit {
        displayLink?.invalidate()
        frameObservation?.cancel()
    }

    override func layoutSubviews() {
        super.layoutSubviews()
        onLayoutChanged?(bounds.size)
        touchHandler.viewSize = bounds.size
    }

    override var canBecomeFirstResponder: Bool { true }

    // MARK: Frame observation using Combine

    private func observeFrames() {
        guard let service = webRTCService else { return }
        // Subscribe to the @Published publisher (avoids needing @objc dynamic)
        frameObservation = service.$currentFrame
            .receive(on: DispatchQueue.main)
            .sink { [weak self] image in
                guard let self = self, let image = image else { return }
                self.setFrame(image: image)
            }
    }

    // MARK: Frame display

    private func setFrame(image: UIImage) {
        imageView.image = image
        frameCount += 1

        if activityIndicator.isAnimating {
            activityIndicator.stopAnimating()
        }
    }

    // MARK: FPS calculation via display link

    @objc private func displayLinkTick(_ link: CADisplayLink) {
        let now = CACurrentMediaTime()
        let elapsed = now - lastFPSTime
        if elapsed >= 1.0 {
            let currentFPS = Int(Double(frameCount) / elapsed)
            frameCount = 0
            lastFPSTime = now
            onFPSUpdate?(currentFPS)
        }
    }

    // MARK: - Gesture Recognizers

    @objc private func handlePinch(_ sender: UIPinchGestureRecognizer) {
        switch sender.state {
        case .began:
            lastPinchScale = currentScale

        case .changed:
            let newScale = (lastPinchScale * sender.scale).clamped(to: minZoomScale...maxZoomScale)
            currentScale = newScale

        default:
            break
        }
    }

    @objc private func handlePan(_ sender: UIPanGestureRecognizer) {
        guard currentScale > 1.0 else {
            // When not zoomed, two-finger pan = scroll
            if sender.state == .changed {
                let translation = sender.translation(in: self)
                sender.setTranslation(.zero, in: self)
                let scale: Double = 3.0
                webRTCService?.sendScroll(
                    deltaX: Double(translation.x) * scale,
                    deltaY: Double(translation.y) * scale
                )
            }
            return
        }

        // When zoomed in, pan translates the image
        switch sender.state {
        case .changed:
            let translation = sender.translation(in: self)
            currentOffset.x += translation.x
            currentOffset.y += translation.y
            sender.setTranslation(.zero, in: self)

        default:
            break
        }
    }

    @objc private func handleDoubleTap(_ sender: UITapGestureRecognizer) {
        let location = sender.location(in: self)
        if currentScale > 1.0 {
            // Zoom out to fit
            animateZoom(scale: 1.0, offset: .zero)
        } else {
            // Zoom in centered on tap point
            let targetScale: CGFloat = 2.0
            let targetOffset = CGPoint(
                x: (bounds.midX - location.x) * (targetScale - 1.0),
                y: (bounds.midY - location.y) * (targetScale - 1.0)
            )
            animateZoom(scale: targetScale, offset: targetOffset)
        }
    }

    @objc private func handleTwoFingerTap(_ sender: UITapGestureRecognizer) {
        guard sender.state == .recognized else { return }
        let location = sender.location(in: self)
        let (mx, my) = touchHandler.convertToMac(location, in: self)
        webRTCService?.sendMouseClick(button: 1, x: mx, y: my, down: true)
        webRTCService?.sendMouseClick(button: 1, x: mx, y: my, down: false)
    }

    // MARK: Zoom animation

    private func animateZoom(scale: CGFloat, offset: CGPoint) {
        UIView.animate(withDuration: 0.25, delay: 0, options: .curveEaseInOut) {
            self.currentScale = scale
            self.currentOffset = offset
            self.layoutIfNeeded()
        }
    }

    // MARK: Transform

    private func applyTransform() {
        var transform = CGAffineTransform.identity
        transform = transform.translatedBy(x: currentOffset.x, y: currentOffset.y)
        transform = transform.scaledBy(x: currentScale, y: currentScale)
        imageView.transform = transform
    }

    // MARK: - Touch Forwarding

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent?) {
        super.touchesBegan(touches, with: event)
        touchHandler.touchesBegan(touches, in: self)
        becomeFirstResponder()
    }

    override func touchesMoved(_ touches: Set<UITouch>, with event: UIEvent?) {
        super.touchesMoved(touches, with: event)
        touchHandler.touchesMoved(touches, in: self)
    }

    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent?) {
        super.touchesEnded(touches, with: event)
        touchHandler.touchesEnded(touches, in: self)
    }

    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent?) {
        super.touchesCancelled(touches, with: event)
        touchHandler.touchesCancelled(touches, in: self)
    }

    // MARK: - Keyboard Input (Hardware)

    override func pressesBegan(_ presses: Set<UIPress>, with event: UIPressesEvent?) {
        var handled = false
        for press in presses {
            guard let key = press.key else { continue }
            let keyCode = keyCodeForUIKey(key)
            touchHandler.keyDown(keyCode: keyCode, chars: key.characters)
            handled = true
        }
        if !handled {
            super.pressesBegan(presses, with: event)
        }
    }

    override func pressesEnded(_ presses: Set<UIPress>, with event: UIPressesEvent?) {
        var handled = false
        for press in presses {
            guard let key = press.key else { continue }
            let keyCode = keyCodeForUIKey(key)
            touchHandler.keyUp(keyCode: keyCode)
            handled = true
        }
        if !handled {
            super.pressesEnded(presses, with: event)
        }
    }

    // MARK: UIKey → macOS virtual key code

    private func keyCodeForUIKey(_ key: UIKey) -> UInt16 {
        // Map UIKeyboardHIDUsage to macOS virtual key codes
        // This uses the characters property as a fallback when HID usage isn't available
        let chars = key.characters
        let char = chars.first ?? Character(" ")

        // Check for special keys
        if key.modifierFlags.contains(.command) { return 0x37 } // kVK_Command
        if key.modifierFlags.contains(.shift) { return 0x38 }  // kVK_Shift
        if key.modifierFlags.contains(.alternate) { return 0x3A } // kVK_Option
        if key.modifierFlags.contains(.control) { return 0x3B } // kVK_Control

        // Map function keys by HID usage
        switch key.keyCode {
        case 0x70000004: return 0x00 // a
        case 0x70000005: return 0x0B // b
        case 0x70000006: return 0x08 // c
        case 0x70000007: return 0x02 // d
        case 0x70000008: return 0x0E // e
        case 0x70000009: return 0x03 // f
        case 0x7000000A: return 0x05 // g
        case 0x7000000B: return 0x04 // h
        case 0x7000000C: return 0x22 // i
        case 0x7000000D: return 0x26 // j
        case 0x7000000E: return 0x28 // k
        case 0x7000000F: return 0x25 // l
        case 0x70000010: return 0x2D // m
        case 0x70000011: return 0x2E // n
        case 0x70000012: return 0x1F // o
        case 0x70000013: return 0x23 // p
        case 0x70000014: return 0x0C // q
        case 0x70000015: return 0x0F // r
        case 0x70000016: return 0x01 // s
        case 0x70000017: return 0x11 // t
        case 0x70000018: return 0x20 // u
        case 0x70000019: return 0x09 // v
        case 0x7000001A: return 0x0D // w
        case 0x7000001B: return 0x07 // x
        case 0x7000001C: return 0x10 // y
        case 0x7000001D: return 0x06 // z
        case 0x7000001E: return 0x12 // 1
        case 0x7000001F: return 0x13 // 2
        case 0x70000020: return 0x14 // 3
        case 0x70000021: return 0x15 // 4
        case 0x70000022: return 0x17 // 5
        case 0x70000023: return 0x16 // 6
        case 0x70000024: return 0x1A // 7
        case 0x70000025: return 0x1C // 8
        case 0x70000026: return 0x19 // 9
        case 0x70000027: return 0x1D // 0
        case 0x70000028: return 0x24 // Return
        case 0x70000029: return 0x35 // Escape
        case 0x7000002A: return 0x33 // Delete (Backspace)
        case 0x7000002B: return 0x30 // Tab
        case 0x7000002C: return 0x31 // Space
        case 0x7000002D: return 0x1B // -
        case 0x7000002E: return 0x18 // =
        case 0x7000002F: return 0x21 // [
        case 0x70000030: return 0x1E // ]
        case 0x70000031: return 0x2A // \
        case 0x70000033: return 0x29 // ;
        case 0x70000034: return 0x27 // '
        case 0x70000035: return 0x32 // `
        case 0x70000036: return 0x2B // ,
        case 0x70000037: return 0x2F // .
        case 0x70000038: return 0x2C // /
        case 0x70000039: return 0x39 // Caps Lock
        case 0x7000003A: return 0x7A // F1
        case 0x7000003B: return 0x78 // F2
        case 0x7000003C: return 0x63 // F3
        case 0x7000003D: return 0x76 // F4
        case 0x7000003E: return 0x60 // F5
        case 0x7000003F: return 0x61 // F6
        case 0x70000040: return 0x62 // F7
        case 0x70000041: return 0x64 // F8
        case 0x70000042: return 0x65 // F9
        case 0x70000043: return 0x6D // F10
        case 0x70000044: return 0x67 // F11
        case 0x70000045: return 0x6F // F12
        case 0x70000046: return 0x69 // Print Screen
        case 0x70000047: return 0x6B // Scroll Lock
        case 0x70000048: return 0x71 // Pause
        case 0x70000049: return 0x73 // Insert
        case 0x7000004A: return 0x77 // Home
        case 0x7000004B: return 0x74 // Page Up
        case 0x7000004C: return 0x75 // Delete (Forward)
        case 0x7000004D: return 0x77 // End
        case 0x7000004E: return 0x79 // Page Down
        case 0x7000004F: return 0x7C // Right Arrow
        case 0x70000050: return 0x7B // Left Arrow
        case 0x70000051: return 0x7D // Down Arrow
        case 0x70000052: return 0x7E // Up Arrow — note: 0x7E is actually Up Arrow on macOS
        default:
            break
        }

        // Fallback: use characters mapping
        return charToKeyCode(char)
    }

    private func charToKeyCode(_ char: Character) -> UInt16 {
        switch char.lowercased().first ?? " " {
        case "a": return 0x00
        case "s": return 0x01
        case "d": return 0x02
        case "f": return 0x03
        case "h": return 0x04
        case "g": return 0x05
        case "z": return 0x06
        case "x": return 0x07
        case "c": return 0x08
        case "v": return 0x09
        case "b": return 0x0B
        case "q": return 0x0C
        case "w": return 0x0D
        case "e": return 0x0E
        case "r": return 0x0F
        case "y": return 0x10
        case "t": return 0x11
        case "1": return 0x12
        case "2": return 0x13
        case "3": return 0x14
        case "4": return 0x15
        case "6": return 0x16
        case "5": return 0x17
        case "=": return 0x18
        case "9": return 0x19
        case "7": return 0x1A
        case "-": return 0x1B
        case "8": return 0x1C
        case "0": return 0x1D
        case "]": return 0x1E
        case "o": return 0x1F
        case "u": return 0x20
        case "[": return 0x21
        case "i": return 0x22
        case "p": return 0x23
        case "\n": return 0x24
        case "l": return 0x25
        case "j": return 0x26
        case "'": return 0x27
        case "k": return 0x28
        case ";": return 0x29
        case "\\": return 0x2A
        case ",": return 0x2B
        case "/": return 0x2C
        case "n": return 0x2D
        case "m": return 0x2E
        case ".": return 0x2F
        case "\t": return 0x30
        case " ": return 0x31
        case "`": return 0x32
        default:  return 0x00 // default to 'a'
        }
    }
}

// MARK: - CGFloat clamping

private extension CGFloat {
    func clamped(to range: ClosedRange<CGFloat>) -> CGFloat {
        return min(max(self, range.lowerBound), range.upperBound)
    }
}

private extension Double {
    func clamped(to range: ClosedRange<Double>) -> Double {
        return min(max(self, range.lowerBound), range.upperBound)
    }
}
