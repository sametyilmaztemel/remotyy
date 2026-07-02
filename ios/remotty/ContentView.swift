import SwiftUI

struct ContentView: View {
    @EnvironmentObject private var app: AppState
    @State private var showConnection = false
    
    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                // Status Bar
                HStack {
                    Circle()
                        .fill(statusColor)
                        .frame(width: 8, height: 8)
                    Text(app.status.rawValue)
                        .font(.caption)
                        .foregroundColor(.secondary)
                    Spacer()
                    Text("remotty")
                        .font(.caption)
                        .foregroundColor(.secondary)
                        .fontDesign(.monospaced)
                }
                .padding(.horizontal)
                .padding(.vertical, 8)
                .background(Color(.systemGray6))
                
                if app.hosts.isEmpty {
                    Spacer()
                    VStack(spacing: 16) {
                        Image(systemName: "terminal.fill")
                            .font(.system(size: 48))
                            .foregroundColor(.secondary)
                        Text("No Hosts Available")
                            .font(.title2)
                            .fontWeight(.semibold)
                        Text("Connect to a signaling server to discover remote hosts")
                            .font(.subheadline)
                            .foregroundColor(.secondary)
                            .multilineTextAlignment(.center)
                        Button(action: { showConnection = true }) {
                            Label("Connect to Server", systemImage: "link")
                                .font(.headline)
                                .padding()
                                .frame(maxWidth: 280)
                                .background(Color.accentColor)
                                .foregroundColor(.white)
                                .clipShape(RoundedRectangle(cornerRadius: 12))
                        }
                    }
                    Spacer()
                } else {
                    List(app.hosts) { host in
                        NavigationLink(destination: TerminalView(host: host)) {
                            HostRow(host: host)
                        }
                        .swipeActions(edge: .trailing) {
                            if host.features.contains("screen") {
                                NavigationLink(destination: ScreenView(host: host)) {
                                    Label("Screen", systemImage: "display")
                                }
                                .tint(.blue)
                            }
                        }
                        .contextMenu {
                            NavigationLink(destination: TerminalView(host: host)) {
                                Label("Terminal", systemImage: "terminal")
                            }
                            if host.features.contains("screen") {
                                NavigationLink(destination: ScreenView(host: host)) {
                                    Label("Screen Share", systemImage: "display")
                                }
                            }
                        }
                    }
                    .listStyle(.plain)
                }
            }
            .navigationTitle("remotty")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button(action: { showConnection = true }) {
                        Image(systemName: "gearshape")
                    }
                }
            }
            .sheet(isPresented: $showConnection) {
                ConnectionView()
            }
        }
    }
    
    private var statusColor: Color {
        switch app.status {
        case .disconnected: return .gray
        case .connecting: return .yellow
        case .connected: return .green
        case .error: return .red
        }
    }
}

struct HostRow: View {
    let host: HostInfo
    
    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 4) {
                Text(host.name)
                    .font(.headline)
                    .fontDesign(.monospaced)
                Text("\(host.platform) / \(host.arch)")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
            Spacer()
            Text(host.features.joined(separator: ", "))
                .font(.caption2)
                .foregroundColor(.secondary)
                .padding(.horizontal, 8)
                .padding(.vertical, 2)
                .background(Color(.systemGray6))
                .clipShape(RoundedRectangle(cornerRadius: 4))
        }
        .padding(.vertical, 4)
    }
}
