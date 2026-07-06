import SwiftUI
import UIKit
import Combine

// MARK: - Terminal View

/// SwiftUI terminal that displays PTY output from the WebRTC terminal data channel
/// and provides a keyboard toolbar with common terminal control sequences (^C, TAB, ESC, arrows).
struct TerminalView: View {
    let host: HostInfo
    @EnvironmentObject private var app: AppState

    // MARK: Local state

    @State private var inputText: String = ""
    @State private var outputText: String = ""
    @State private var connectionState: WebRTCState = .disconnected
    @State private var cancellables = Set<AnyCancellable>()
    @FocusState private var isInputFocused: Bool

    // MARK: Body

    var body: some View {
        VStack(spacing: 0) {
            // Terminal Output Area
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 0) {
                        Text(outputText.isEmpty ? placeholderText : outputText)
                            .font(.system(.body, design: .monospaced))
                            .foregroundColor(outputColor)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .textSelection(.enabled)
                            .padding(8)
                            .id("bottom")
                    }
                }
                .background(Color(white: 0.05))
                .onChange(of: outputText) { _ in
                    withAnimation(.easeOut(duration: 0.1)) {
                        proxy.scrollTo("bottom", anchor: .bottom)
                    }
                }
                .onTapGesture {
                    isInputFocused = true
                }
            }

            Divider()

            // Input Bar
            HStack(spacing: 8) {
                TextField("Enter command...", text: $inputText)
                    .font(.system(.body, design: .monospaced))
                    .textFieldStyle(.plain)
                    .autocapitalization(.none)
                    .disableAutocorrection(true)
                    .onSubmit(sendCommand)
                    .disabled(connectionState != .connected)
                    .focused($isInputFocused)

                Button(action: sendCommand) {
                    Image(systemName: "arrow.up.circle.fill")
                        .font(.title2)
                }
                .disabled(inputText.isEmpty || connectionState != .connected)
            }
            .padding()
            .background(Color(.systemGray6))
        }
        .navigationTitle(host.name)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            // Connection status indicator (top-right)
            ToolbarItem(placement: .topBarTrailing) {
                HStack(spacing: 6) {
                    Circle()
                        .fill(connectionIndicatorColor)
                        .frame(width: 8, height: 8)
                    Text(connectionIndicatorText)
                        .font(.caption2)
                }
            }

            // Keyboard toolbar with common terminal control sequences
            ToolbarItemGroup(placement: .keyboard) {
                keyboardToolbarContent
            }
        }
        .onAppear {
            connectToHost()
            observeWebRTCService()

            // Auto-focus input shortly after appearing
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {
                isInputFocused = true
            }
        }
        .onDisappear {
            cancellables.removeAll()
            disconnectFromHost()
        }
    }

    // MARK: - Keyboard Toolbar

    /// Content shown above the software keyboard when the text field is active.
    @ViewBuilder
    private var keyboardToolbarContent: some View {
        Group {
            ctrlCButton
            Spacer()
            tabButton
            Spacer()
            escButton
            Spacer()
            arrowButtons
            Spacer()
            dismissKeyboardButton
        }
        .disabled(connectionState != .connected)
        .buttonStyle(.plain)
    }

    /// Send Ctrl+C (ETX — \x03) to interrupt the foreground process.
    private var ctrlCButton: some View {
        Button(action: { sendControlSequence("\u{0003}") }) {
            Text("^C")
                .font(.caption.bold())
                .padding(.horizontal, 10)
                .padding(.vertical, 6)
                .background(Color.red.opacity(0.25))
                .cornerRadius(6)
        }
    }

    /// Send a literal TAB character.
    private var tabButton: some View {
        Button(action: { sendControlSequence("\t") }) {
            Text("TAB")
                .font(.caption.bold())
                .padding(.horizontal, 10)
                .padding(.vertical, 6)
                .background(Color.blue.opacity(0.25))
                .cornerRadius(6)
        }
    }

    /// Send ESC (Escape — \x1b).
    private var escButton: some View {
        Button(action: { sendControlSequence("\u{001b}") }) {
            Text("ESC")
                .font(.caption.bold())
                .padding(.horizontal, 8)
                .padding(.vertical, 6)
                .background(Color.orange.opacity(0.25))
                .cornerRadius(6)
        }
    }

    /// Arrow key cluster: up / down / left / right.
    private var arrowButtons: some View {
        HStack(spacing: 4) {
            arrowButton(systemName: "arrowtriangle.up.fill",    sequence: "\u{001b}[A")
            arrowButton(systemName: "arrowtriangle.down.fill",  sequence: "\u{001b}[B")
            arrowButton(systemName: "arrowtriangle.left.fill",  sequence: "\u{001b}[D")
            arrowButton(systemName: "arrowtriangle.right.fill", sequence: "\u{001b}[C")
        }
    }

    private func arrowButton(systemName: String, sequence: String) -> some View {
        Button(action: { sendControlSequence(sequence) }) {
            Image(systemName: systemName)
                .font(.caption)
                .padding(8)
                .background(Color.gray.opacity(0.2))
                .cornerRadius(6)
        }
    }

    /// Dismiss the software keyboard.
    private var dismissKeyboardButton: some View {
        Button(action: { isInputFocused = false }) {
            Image(systemName: "keyboard.chevron.compact.down")
                .font(.caption)
                .foregroundColor(.secondary)
        }
    }

    // MARK: - Sending Data

    /// Send a control sequence (raw bytes) over the terminal data channel.
    private func sendControlSequence(_ sequence: String) {
        app.webRTCService.sendTerminalInput(sequence)
    }

    /// Send the current input text as a command (appends newline).
    private func sendCommand() {
        guard !inputText.isEmpty, connectionState == .connected else { return }
        let cmd = inputText + "\n"
        app.webRTCService.sendTerminalInput(cmd)
        inputText = ""
    }

    /// Notify the remote host of the terminal size.
    private func sendTerminalResize() {
        app.webRTCService.sendTerminalResize(rows: 24, cols: 80)
    }

    // MARK: - Connection Lifecycle

    private func connectToHost() {
        connectionState = .connecting
        app.webRTCService.connect(
            signalURL: app.signalURL,
            hostID: host.id,
            password: nil
        )
    }

    private func disconnectFromHost() {
        app.webRTCService.disconnect()
    }

    // MARK: - Combine Observation

    /// Subscribe to `WebRTCService` publishers so the view stays in sync
    /// without polling.
    private func observeWebRTCService() {
        // Connection state
        app.webRTCService.$state
            .receive(on: DispatchQueue.main)
            .sink { [weak self] state in
                guard let self = self else { return }
                self.connectionState = state
                if state == .connected {
                    self.sendTerminalResize()
                    // Auto-focus when connection is established
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
                        self.isInputFocused = true
                    }
                }
            }
            .store(in: &cancellables)

        // Terminal output — the service appends incoming text to
        // `terminalOutput` as it arrives, so we just mirror it.
        app.webRTCService.$terminalOutput
            .receive(on: DispatchQueue.main)
            .sink { [weak self] output in
                self?.outputText = output
            }
            .store(in: &cancellables)
    }

    // MARK: - Helpers

    private var placeholderText: String {
        switch connectionState {
        case .connecting:
            return "Connecting to \(host.name)...\n"
        case .disconnected:
            return "Disconnected from \(host.name)\n"
        case .error(let msg):
            return "Error: \(msg)\n"
        case .connected:
            return "Connected to \(host.name)\n"
        }
    }

    private var outputColor: Color {
        connectionState == .connected ? Color(white: 0.85) : Color(white: 0.45)
    }

    private var connectionIndicatorColor: Color {
        switch connectionState {
        case .disconnected: return .gray
        case .connecting:   return .yellow
        case .connected:    return .green
        case .error:        return .red
        }
    }

    private var connectionIndicatorText: String {
        switch connectionState {
        case .disconnected: return "Disconnected"
        case .connecting:   return "Connecting..."
        case .connected:    return "Connected"
        case .error(let msg): return msg
        }
    }
}
