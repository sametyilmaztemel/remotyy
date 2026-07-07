import SwiftUI

@main
struct remottyApp: App {
    @StateObject private var appState = AppState()
    
    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(appState)
                .preferredColorScheme(.dark)
        }
    }
}

class AppState: ObservableObject {
    @Published var status: ConnectionStatus = .disconnected
    @Published var hosts: [HostInfo] = []
    @Published var signalURL: String = "ws://localhost:9000"
    
    /// Shared WebRTC service for the current session.
    /// Used by both TerminalView and ScreenView.
    let webRTCService = WebRTCService()
    
    enum ConnectionStatus: String {
        case disconnected = "Disconnected"
        case connecting = "Connecting..."
        case connected = "Connected"
        case error = "Error"
    }
    
    /// Convenience: returns a host by its ID.
    func host(with id: String) -> HostInfo? {
        hosts.first(where: { $0.id == id })
    }
}

struct HostInfo: Identifiable, Codable {
    let id: String
    let name: String
    let platform: String
    let arch: String
    let online: Bool
    let features: [String]
}
