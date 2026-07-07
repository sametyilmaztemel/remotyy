import Foundation
import WebRTC
import UIKit

// MARK: - Service State & Delegate

/// Connection and operational state of the WebRTC service.
enum WebRTCState: Equatable {
    case disconnected
    case connecting
    case connected
    case error(String)

    static func == (lhs: WebRTCState, rhs: WebRTCState) -> Bool {
        switch (lhs, rhs) {
        case (.disconnected, .disconnected): return true
        case (.connecting, .connecting): return true
        case (.connected, .connected): return true
        case (.error(let a), .error(let b)): return a == b
        default: return false
        }
    }
}

/// Callbacks for frame delivery, state changes, terminal output, and errors.
protocol WebRTCServiceDelegate: AnyObject {
    func webRTCService(_ service: WebRTCService, didReceiveFrame image: UIImage)
    func webRTCService(_ service: WebRTCService, didChangeState state: WebRTCState)
    func webRTCService(_ service: WebRTCService, didReceiveTerminalOutput text: String)
    func webRTCService(_ service: WebRTCService, didReceiveError error: Error)
}

// MARK: - WebRTC Connection Manager

/// Manages a WebRTC peer connection to a remote host via a signaling WebSocket.
///
/// Flow:
///  1. Connect to signaling server → 2. Request room with host_id → 3. Receive offer
///  → 4. Create answer → 5. Exchange ICE → 6. Data channels open → 7. Frames & I/O
final class WebRTCService: NSObject, ObservableObject {

    // MARK: Data channel labels (mirrors Go side)
    enum DataChannelLabel: String, CaseIterable {
        case terminal = "terminal"
        case screen   = "screen"
        case auth     = "auth"
        case file     = "file"
    }

    // MARK: Public state

    /// Observable connection state — drives SwiftUI overlays.
    @Published private(set) var state: WebRTCState = .disconnected

    /// The latest decoded screen frame.
    @Published private(set) var currentFrame: UIImage?

    /// Remote display dimensions (may change after connection).
    @Published private(set) var remoteScreenSize: CGSize = CGSize(width: 1920, height: 1080)

    /// Receive terminal output.
    @Published private(set) var terminalOutput: String = ""

    weak var delegate: WebRTCServiceDelegate?

    // MARK: Configuration

    private var signalURL: URL?
    private var hostID: String = ""
    private var password: String?

    // MARK: WebSocket

    private var webSocket: URLSessionWebSocketTask?
    private var webSocketSession: URLSession?
    private var webSocketConnected = false

    // MARK: WebRTC

    private var factory: RTCPeerConnectionFactory?
    private var peerConnection: RTCPeerConnection?
    private var dataChannels: [DataChannelLabel: RTCDataChannel] = [:]
    private var pendingCandidates: [RTCIceCandidate] = []

    // MARK: Reconnection

    private var reconnectAttempts = 0
    private let maxReconnectAttempts = 5
    private var isReconnecting = false
    private var reconnectTimer: Timer?

    // MARK: Frame coalescing

    private var lastFrameTimestamp: CFTimeInterval = 0
    private var lastFrameDelivery: CFTimeInterval = 0
    private let minFrameInterval: CFTimeInterval = 1.0 / 30.0 // cap at 30 FPS

    // MARK: ICE servers

    private var iceServers: [RTCIceServer] {
        [
            RTCIceServer(urlStrings: ["stun:stun.l.google.com:19302"]),
            RTCIceServer(urlStrings: ["stun:stun1.l.google.com:19302"]),
        ]
    }

    // MARK: Lifecycle

    override init() {
        super.init()
    }

    deinit {
        disconnect()
    }

    // MARK: Public API

    /// Establish a full WebRTC session with a remote host.
    /// - Parameters:
    ///   - signalURL: WebSocket URL of the signaling server.
    ///   - hostID:   The host identifier to connect to.
    ///   - password: Optional master password for authentication.
    func connect(signalURL: String, hostID: String, password: String? = nil) {
        guard let url = URL(string: signalURL) else {
            updateState(.error("Invalid signaling URL"))
            return
        }
        self.signalURL = url
        self.hostID = hostID
        self.password = password
        self.reconnectAttempts = 0
        self.terminalOutput = ""

        startSignaling()
    }

    /// Gracefully tear down the connection.
    func disconnect() {
        reconnectTimer?.invalidate()
        reconnectTimer = nil
        isReconnecting = false

        webSocket?.cancel(with: .normalClosure, reason: nil)
        webSocket = nil
        webSocketSession = nil
        webSocketConnected = false

        peerConnection?.close()
        peerConnection = nil
        dataChannels.removeAll()
        pendingCandidates.removeAll()
        factory = nil

        updateState(.disconnected)
    }

    // MARK: Sending data

    /// Send raw data over a named data channel.
    func send(data: Data, channel: DataChannelLabel) {
        guard let dataChannel = dataChannels[channel] else {
            log("[WebRTC] No data channel '\(channel.rawValue)' — dropping send")
            return
        }
        let buffer = RTCDataBuffer(data: data, isBinary: true)
        dataChannel.sendData(buffer)
    }

    /// Send a JSON-serialisable dictionary over a data channel.
    func send(json: [String: Any], channel: DataChannelLabel) {
        guard let data = try? JSONSerialization.data(withJSONObject: json, options: [.sortedKeys]) else {
            log("[WebRTC] Failed to serialise JSON for channel '\(channel.rawValue)'")
            return
        }
        send(data: data, channel: channel)
    }

    /// Send a protocol‑envelope message: `{"type": "...", "payload": {...}}`.
    func sendProtocol(type: String, payload: [String: Any] = [:], channel: DataChannelLabel) {
        var msg: [String: Any] = ["type": type]
        if !payload.isEmpty {
            msg["payload"] = payload
        }
        send(json: msg, channel: channel)
    }

    // MARK: Convenience — input events

    /// Send a mouse-move event (relative to remote screen dimensions).
    func sendMouseMove(x: Double, y: Double) {
        sendProtocol(type: "mouse_move", payload: ["x": x, "y": y], channel: .screen)
    }

    /// Send a mouse click (button: 0=left, 1=right, 2=middle).
    func sendMouseClick(button: Int, x: Double, y: Double, down: Bool) {
        sendProtocol(type: "mouse_click", payload: ["button": button, "x": x, "y": y, "down": down], channel: .screen)
    }

    /// Send a scroll event.
    func sendScroll(deltaX: Double, deltaY: Double) {
        sendProtocol(type: "mouse_scroll", payload: ["delta_x": deltaX, "delta_y": deltaY], channel: .screen)
    }

    /// Send a keyboard event.
    func sendKey(keyCode: UInt16, chars: String? = nil, down: Bool) {
        var payload: [String: Any] = ["key_code": keyCode]
        if let chars = chars, !chars.isEmpty {
            payload["chars"] = chars
        }
        let type = down ? "key_press" : "key_release"
        sendProtocol(type: type, payload: payload, channel: .screen)
    }

    /// Send auth message over the auth channel.
    func sendAuth(password: String) {
        sendProtocol(type: "auth", payload: ["password": password], channel: .auth)
    }

    /// Send terminal input (raw bytes or JSON input envelope).
    func sendTerminalInput(_ text: String) {
        guard let data = text.data(using: .utf8) else { return }
        send(data: data, channel: .terminal)
    }

    /// Send terminal resize notification.
    func sendTerminalResize(rows: UInt16, cols: UInt16) {
        sendProtocol(type: "resize", payload: ["rows": rows, "cols": cols], channel: .terminal)
    }

    /// Request the host to start screen sharing.
    func requestScreenStart(fps: Int = 15, quality: Int = 60) {
        sendProtocol(type: "screen_start", payload: ["fps": fps, "quality": quality], channel: .screen)
    }

    /// Request the host to stop screen sharing.
    func requestScreenStop() {
        sendProtocol(type: "screen_stop", payload: [:], channel: .screen)
    }

    // MARK: - Signaling

    private func startSignaling() {
        guard let url = signalURL else {
            updateState(.error("No signaling URL configured"))
            return
        }

        updateState(.connecting)

        let wsURL: URL
        if url.path.contains("/ws") || url.lastPathComponent == "ws" {
            wsURL = url
        } else {
            // Append /ws path
            guard var components = URLComponents(url: url, resolvingAgainstBaseURL: false) else {
                updateState(.error("Invalid signaling URL"))
                return
            }
            components.path = (components.path.hasSuffix("/") ? "" : "/") + "ws"
            wsURL = components.url ?? url.appendingPathComponent("ws")
        }

        let session = URLSession(configuration: .default, delegate: self, delegateQueue: nil)
        self.webSocketSession = session
        let task = session.webSocketTask(with: wsURL)
        self.webSocket = task
        task.resume()

        receiveWebSocketMessage()
        log("[WebRTC] WebSocket connecting to \(wsURL.absoluteString)")
    }

    // MARK: WebSocket receive loop

    private func receiveWebSocketMessage() {
        webSocket?.receive { [weak self] result in
            guard let self = self else { return }
            switch result {
            case .success(let message):
                switch message {
                case .string(let text):
                    self.handleSignalingText(text)
                case .data(let data):
                    if let text = String(data: data, encoding: .utf8) {
                        self.handleSignalingText(text)
                    }
                @unknown default:
                    break
                }
                self.receiveWebSocketMessage() // continue loop
            case .failure(let error):
                log("[WebRTC] WebSocket receive error: \(error.localizedDescription)")
                scheduleReconnect()
            }
        }
    }

    // MARK: Signaling message handler

    private func handleSignalingText(_ text: String) {
        guard let data = text.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = json["type"] as? String else {
            return
        }

        switch type {

        case "room_ready":
            log("[WebRTC] Room ready — initialising WebRTC")
            initializeWebRTCIfNeeded()

        case "offer":
            guard let payload = json["payload"] as? [String: Any],
                  let sdp = payload["sdp"] as? String else {
                updateState(.error("Invalid offer payload"))
                return
            }
            handleOffer(sdp: sdp, room: json["room"] as? String)

        case "answer":
            // Client receives answer only if it initiated the offer (not our flow normally)
            guard let payload = json["payload"] as? [String: Any],
                  let sdp = payload["sdp"] as? String else { return }
            handleAnswer(sdp: sdp)

        case "ice_candidate":
            guard let payload = json["payload"] as? [String: Any],
                  let candidate = payload["candidate"] as? String,
                  let sdpMid = payload["sdpMid"] as? String,
                  let sdpMLineIndex = payload["sdpMLineIndex"] as? Int32 else { return }
            let iceCandidate = RTCIceCandidate(sdp: candidate, sdpMLineIndex: sdpMLineIndex, sdpMid: sdpMid)
            handleIceCandidate(iceCandidate)

        case "host_list":
            // Handled at the app level (ConnectionView), not here
            break

        case "error":
            if let payload = json["payload"] as? [String: Any],
               let message = payload["message"] as? String {
                updateState(.error(message))
            } else if let message = json["message"] as? String {
                updateState(.error(message))
            }

        case "peer_left":
            log("[WebRTC] Peer disconnected")
            updateState(.disconnected)

        default:
            log("[WebRTC] Unhandled signaling message type: \(type)")
        }
    }

    // MARK: WebRTC negotiation

    private func initializeWebRTCIfNeeded() {
        guard factory == nil else { return }

        log("[WebRTC] Initialising peer connection factory")
        factory = RTCPeerConnectionFactory()

        let config = RTCConfiguration()
        config.iceServers = iceServers
        config.sdpSemantics = .unifiedPlan
        config.continualGatheringPolicy = .gatherOnce
        config.bundlePolicy = .maxBundle
        config.rtcpMuxPolicy = .require

        let constraints = RTCMediaConstraints(
            mandatoryConstraints: [
                "OfferToReceiveAudio": "false",
                "OfferToReceiveVideo": "false",
            ],
            optionalConstraints: nil
        )

        peerConnection = factory?.peerConnection(with: config, constraints: constraints, delegate: self)

        // Flush any pending candidates
        for candidate in pendingCandidates {
            peerConnection?.add(candidate)
        }
        pendingCandidates.removeAll()

        // Connect request was already sent in didOpenWithProtocol
        log("[WebRTC] Peer connection factory initialised, waiting for offer")
    }

    private func sendConnectRequest() {
        let connectPayload: [String: Any] = [
            "host_id": hostID,
        ]
        var msg: [String: Any] = [
            "type": "connect",
            "payload": connectPayload,
        ]
        if let password = password, !password.isEmpty {
            var p = connectPayload
            p["password"] = password
            msg["payload"] = p
        }
        sendWebSocketMessage(msg)
        log("[WebRTC] Sent connect request for host '\(hostID)'")
    }

    private func handleOffer(sdp: String, room: String?) {
        log("[WebRTC] Received offer, creating answer")

        let remoteSdp = RTCSessionDescription(type: .offer, sdp: sdp)
        peerConnection?.setRemoteDescription(remoteSdp) { [weak self] error in
            guard let self = self else { return }

            if let error = error {
                self.updateState(.error("setRemoteDescription(offer): \(error.localizedDescription)"))
                return
            }

            let constraints = RTCMediaConstraints(
                mandatoryConstraints: [
                    "OfferToReceiveAudio": "false",
                    "OfferToReceiveVideo": "false",
                ],
                optionalConstraints: nil
            )

            self.peerConnection?.answer(for: constraints) { [weak self] answer, error in
                guard let self = self else { return }

                if let error = error {
                    self.updateState(.error("createAnswer: \(error.localizedDescription)"))
                    return
                }

                guard let answer = answer else {
                    self.updateState(.error("Empty answer"))
                    return
                }

                self.peerConnection?.setLocalDescription(answer) { [weak self] error in
                    guard let self = self else { return }

                    if let error = error {
                        self.updateState(.error("setLocalDescription(answer): \(error.localizedDescription)"))
                        return
                    }

                    // Create data channels now that local description is set
                    self.setupDataChannels()

                    // Send answer via signaling
                    let answerPayload: [String: Any] = [
                        "type": "answer",
                        "sdp": answer.sdp,
                    ]
                    var msg: [String: Any] = [
                        "type": "answer",
                        "payload": answerPayload,
                    ]
                    if let room = room {
                        msg["room"] = room
                    }
                    self.sendWebSocketMessage(msg)
                    log("[WebRTC] Sent answer")
                }
            }
        }
    }

    private func handleAnswer(sdp: String) {
        let remoteSdp = RTCSessionDescription(type: .answer, sdp: sdp)
        peerConnection?.setRemoteDescription(remoteSdp) { [weak self] error in
            if let error = error {
                self?.updateState(.error("setRemoteDescription(answer): \(error.localizedDescription)"))
            } else {
                self?.log("[WebRTC] Remote description set (answer)")
            }
        }
    }

    private func handleIceCandidate(_ candidate: RTCIceCandidate) {
        if let pc = peerConnection {
            pc.add(candidate)
        } else {
            pendingCandidates.append(candidate)
        }
    }

    // MARK: WebSocket send

    private func sendWebSocketMessage(_ message: [String: Any]) {
        guard let data = try? JSONSerialization.data(withJSONObject: message, options: [.sortedKeys]),
              let text = String(data: data, encoding: .utf8) else {
            log("[WebRTC] Failed to serialise WebSocket message")
            return
        }
        webSocket?.send(.string(text)) { [weak self] error in
            if let error = error {
                self?.log("[WebRTC] WebSocket send error: \(error.localizedDescription)")
            }
        }
    }

    // MARK: Data channels

    /// Create all four data channels after the peer connection is established.
    private func setupDataChannels() {
        for label in DataChannelLabel.allCases {
            let config = RTCDataChannelConfiguration()
            config.isOrdered = true
            config.isNegotiated = false
            // No explicit channelId — WebRTC assigns dynamically when isNegotiated is false

            if let channel = peerConnection?.dataChannel(forLabel: label.rawValue, configuration: config) {
                channel.delegate = self
                dataChannels[label] = channel
                log("[WebRTC] Created data channel: \(label.rawValue)")
            }
        }

        // Send auth if password provided
        if let password = password, !password.isEmpty {
            sendAuth(password: password)
        }

        // Request screen sharing start
        requestScreenStart()
    }

    // MARK: Screen frame handling

    private func handleScreenFrame(data: Data) {
        guard let image = UIImage(data: data) else {
            log("[WebRTC] Failed to decode JPEG frame (\(data.count) bytes)")
            return
        }
        let now = CACurrentMediaTime()
        // Coalesce: skip frames arriving faster than our display can handle
        guard now - lastFrameDelivery >= minFrameInterval else { return }
        lastFrameDelivery = now

        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            self.currentFrame = image
            self.delegate?.webRTCService(self, didReceiveFrame: image)
        }
    }

    private func decodeBase64Frame(_ base64: String) {
        guard let data = Data(base64Encoded: base64) else {
            log("[WebRTC] Invalid base64 frame payload (\(base64.count) chars)")
            return
        }
        handleScreenFrame(data: data)
    }

    // MARK: State management

    private func updateState(_ newState: WebRTCState) {
        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            guard self.state != newState else { return }
            self.state = newState
            self.delegate?.webRTCService(self, didChangeState: newState)
            self.log("[WebRTC] State → \(newState)")
        }
    }

    // MARK: Reconnection

    private func scheduleReconnect() {
        guard !isReconnecting, reconnectAttempts < maxReconnectAttempts else {
            updateState(.error("Max reconnection attempts (\(maxReconnectAttempts)) reached"))
            return
        }

        isReconnecting = true
        reconnectAttempts += 1

        let delay = min(pow(2.0, Double(reconnectAttempts)), 30.0)
        log("[WebRTC] Reconnecting in \(String(format: "%.1f", delay))s (attempt \(reconnectAttempts)/\(maxReconnectAttempts))")

        updateState(.connecting)

        reconnectTimer = Timer.scheduledTimer(withTimeInterval: delay, repeats: false) { [weak self] _ in
            guard let self = self else { return }
            self.isReconnecting = false
            self.startSignaling()
        }
    }

    // MARK: Logging

    private func log(_ message: String) {
        #if DEBUG
        print(message)
        #endif
    }
}

// MARK: - URLSessionWebSocketDelegate

extension WebRTCService: URLSessionWebSocketDelegate {
    func urlSession(
        _ session: URLSession,
        webSocketTask: URLSessionWebSocketTask,
        didOpenWithProtocol protocol: String?
    ) {
        webSocketConnected = true
        log("[WebRTC] WebSocket connected (protocol: \(`protocol` ?? "none"))")
        // Send connect request immediately — don't wait for room_ready
        sendConnectRequest()
    }

    func urlSession(
        _ session: URLSession,
        webSocketTask: URLSessionWebSocketTask,
        didCloseWith closeCode: URLSessionWebSocketTask.CloseCode,
        reason: Data?
    ) {
        webSocketConnected = false
        let reasonStr = reason.flatMap { String(data: $0, encoding: .utf8) } ?? "—"
        log("[WebRTC] WebSocket closed: code=\(closeCode.rawValue) reason=\(reasonStr)")
        scheduleReconnect()
    }
}

// MARK: - RTCPeerConnectionDelegate

extension WebRTCService: RTCPeerConnectionDelegate {
    func peerConnection(_ peerConnection: RTCPeerConnection, didChange stateChanged: RTCSignalingState) {
        log("[WebRTC] Signaling state: \(stateChanged.rawValue)")
    }

    func peerConnection(_ peerConnection: RTCPeerConnection, didAdd stream: RTCMediaStream) {
        log("[WebRTC] Media stream added")
    }

    func peerConnection(_ peerConnection: RTCPeerConnection, didRemove stream: RTCMediaStream) {
        log("[WebRTC] Media stream removed")
    }

    func peerConnectionShouldNegotiate(_ peerConnection: RTCPeerConnection) {
        log("[WebRTC] Negotiation needed")
    }

    func peerConnection(_ peerConnection: RTCPeerConnection, didChange newState: RTCIceConnectionState) {
        log("[WebRTC] ICE connection state: \(newState.rawValue)")
        switch newState {
        case .connected, .completed:
            updateState(.connected)
        case .disconnected:
            log("[WebRTC] ICE disconnected — will attempt reconnection")
        case .failed:
            updateState(.error("ICE connection failed"))
            scheduleReconnect()
        case .closed:
            updateState(.disconnected)
        default:
            break
        }
    }

    func peerConnection(_ peerConnection: RTCPeerConnection, didChange newState: RTCIceGatheringState) {
        log("[WebRTC] ICE gathering state: \(newState.rawValue)")
    }

    func peerConnection(_ peerConnection: RTCPeerConnection, didGenerate candidate: RTCIceCandidate) {
        let candidatePayload: [String: Any] = [
            "candidate": candidate.sdp,
            "sdpMid": candidate.sdpMid ?? "",
            "sdpMLineIndex": candidate.sdpMLineIndex,
        ]
        var msg: [String: Any] = [
            "type": "ice_candidate",
            "payload": candidatePayload,
        ]
        sendWebSocketMessage(msg)
    }

    func peerConnection(_ peerConnection: RTCPeerConnection, didRemove candidates: [RTCIceCandidate]) {
        // No-op for our use case
    }

    func peerConnection(_ peerConnection: RTCPeerConnection, didOpen dataChannel: RTCDataChannel) {
        log("[WebRTC] Data channel opened: \(dataChannel.label)")
        if let label = DataChannelLabel(rawValue: dataChannel.label) {
            dataChannel.delegate = self
            dataChannels[label] = dataChannel
        }
    }
}

// MARK: - RTCDataChannelDelegate

extension WebRTCService: RTCDataChannelDelegate {
    func dataChannelDidChangeState(_ dataChannel: RTCDataChannel) {
        log("[WebRTC] Data channel '\(dataChannel.label)' state: \(dataChannel.readyState.rawValue)")

        // When all expected channels are open, we consider the session ready
        if dataChannel.readyState == .open,
           let label = DataChannelLabel(rawValue: dataChannel.label),
           label == .screen {
            // Request screen sharing once the screen channel is open
            requestScreenStart()
        }
    }

    func dataChannel(_ dataChannel: RTCDataChannel, didReceiveMessageWith buffer: RTCDataBuffer) {
        let data = buffer.data
        guard let label = DataChannelLabel(rawValue: dataChannel.label) else {
            log("[WebRTC] Unknown data channel label: \(dataChannel.label)")
            return
        }

        switch label {
        case .screen:
            handleScreenChannelMessage(data)
        case .terminal:
            handleTerminalChannelMessage(data)
        case .auth:
            handleAuthChannelMessage(data)
        case .file:
            handleFileChannelMessage(data)
        }
    }

    // MARK: Screen channel

    private func handleScreenChannelMessage(_ data: Data) {
        // Messages are JSON envelopes: {"type": "...", "payload": {...}}
        // Screen frames could also be raw JPEG bytes (fast path)
        guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = json["type"] as? String else {
            // Assume raw JPEG data (fast path fallback)
            handleScreenFrame(data: data)
            return
        }

        switch type {
        case "screen_frame":
            if let payload = json["payload"] as? [String: Any],
               let base64 = payload["data"] as? String {
                decodeBase64Frame(base64)
            } else if let base64 = json["payload"] as? String {
                decodeBase64Frame(base64)
            }

        case "screen_resize":
            if let payload = json["payload"] as? [String: Any],
               let width = payload["width"] as? Double,
               let height = payload["height"] as? Double {
                DispatchQueue.main.async { [weak self] in
                    self?.remoteScreenSize = CGSize(width: width, height: height)
                }
            }

        default:
            log("[WebRTC] Unhandled screen message: \(type)")
        }
    }

    // MARK: Terminal channel

    private func handleTerminalChannelMessage(_ data: Data) {
        // Terminal output can be raw bytes (fast path) or JSON envelope
        if let text = String(data: data, encoding: .utf8) {
            // Check if it's a protocol envelope
            if let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
               let type = json["type"] as? String {
                switch type {
                case "output":
                    if let payload = json["payload"] as? String {
                        appendTerminalOutput(payload)
                    }
                default:
                    // Unknown JSON type — forward raw
                    appendTerminalOutput(text)
                }
            } else {
                // Raw text output
                appendTerminalOutput(text)
            }
        } else {
            // Binary data — try UTF-8 anyway
            if let text = String(data: data, encoding: .utf8) {
                appendTerminalOutput(text)
            }
        }
    }

    private func appendTerminalOutput(_ text: String) {
        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }
            self.terminalOutput += text
            self.delegate?.webRTCService(self, didReceiveTerminalOutput: text)
        }
    }

    // MARK: Auth channel

    private func handleAuthChannelMessage(_ data: Data) {
        guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = json["type"] as? String else { return }

        switch type {
        case "auth_ok":
            log("[WebRTC] Authentication successful")
        case "auth_fail":
            updateState(.error("Authentication failed"))
        default:
            break
        }
    }

    // MARK: File channel

    private func handleFileChannelMessage(_ data: Data) {
        // File transfer — delegated to future implementation
        log("[WebRTC] File channel message (\(data.count) bytes) — not yet implemented")
    }
}
