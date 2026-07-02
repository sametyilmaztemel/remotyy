import SwiftUI

struct ConnectionView: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss
    @State private var signalURL: String = ""
    @State private var password: String = ""
    
    var body: some View {
        NavigationStack {
            Form {
                Section("Signaling Server") {
                    TextField("ws://host:port", text: $signalURL)
                        .fontDesign(.monospaced)
                        .autocapitalization(.none)
                        .disableAutocorrection(true)
                }
                
                Section("Authentication") {
                    SecureField("Master Password (optional)", text: $password)
                        .fontDesign(.monospaced)
                }
                
                Section {
                    Button(action: connect) {
                        HStack {
                            Spacer()
                            Text("Connect")
                                .fontWeight(.semibold)
                            Spacer()
                        }
                    }
                    .disabled(signalURL.isEmpty)
                }
            }
            .navigationTitle("Connection")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
            .onAppear {
                signalURL = app.signalURL
            }
        }
    }
    
    private func connect() {
        app.signalURL = signalURL
        app.status = .connecting

        // Connect to the signaling server WebSocket, list hosts, then close
        let wsURL: String
        if signalURL.hasSuffix("/ws") {
            wsURL = signalURL
        } else if signalURL.hasSuffix("/") {
            wsURL = signalURL + "ws"
        } else {
            wsURL = signalURL + "/ws"
        }

        guard let url = URL(string: wsURL) else {
            app.status = .error
            return
        }

        let session = URLSession(configuration: .default)
        let task = session.webSocketTask(with: url)
        task.resume()

        // Send list_hosts message
        let listMsg: [String: Any] = ["type": "list_hosts"]
        guard let listData = try? JSONSerialization.data(withJSONObject: listMsg, options: [.sortedKeys]),
              let listText = String(data: listData, encoding: .utf8) else {
            app.status = .error
            return
        }
        task.send(.string(listText)) { error in
            if let error = error {
                DispatchQueue.main.async {
                    self.app.status = .error
                    print("[Connection] WebSocket send error: \(error.localizedDescription)")
                }
                task.cancel()
                return
            }
        }

        // Receive host_list response
        task.receive { result in
            switch result {
            case .success(let message):
                let text: String
                switch message {
                case .string(let s): text = s
                case .data(let d): text = String(data: d, encoding: .utf8) ?? ""
                @unknown default: text = ""
                }

                guard let data = text.data(using: .utf8),
                      let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                      let type = json["type"] as? String else {
                    DispatchQueue.main.async {
                        self.app.status = .error
                    }
                    task.cancel()
                    return
                }

                if type == "host_list" {
                    if let payload = json["payload"] as? [String: Any],
                       let hostsArray = payload["hosts"] as? [[String: Any]] {
                        let hosts = hostsArray.compactMap { h -> HostInfo? in
                            guard let id = h["id"] as? String,
                                  let name = h["name"] as? String else { return nil }
                            return HostInfo(
                                id: id,
                                name: name,
                                platform: h["platform"] as? String ?? "",
                                arch: h["arch"] as? String ?? "",
                                online: h["online"] as? Bool ?? false,
                                features: h["features"] as? [String] ?? []
                            )
                        }
                        DispatchQueue.main.async {
                            self.app.status = hosts.isEmpty ? .error : .connected
                            self.app.hosts = hosts
                            self.dismiss()
                        }
                    } else {
                        // Try without payload wrapper
                        if let hostsArray = json["hosts"] as? [[String: Any]] {
                            let hosts = hostsArray.compactMap { h -> HostInfo? in
                                guard let id = h["id"] as? String,
                                      let name = h["name"] as? String else { return nil }
                                return HostInfo(
                                    id: id,
                                    name: name,
                                    platform: h["platform"] as? String ?? "",
                                    arch: h["arch"] as? String ?? "",
                                    online: h["online"] as? Bool ?? false,
                                    features: h["features"] as? [String] ?? []
                                )
                            }
                            DispatchQueue.main.async {
                                self.app.status = hosts.isEmpty ? .error : .connected
                                self.app.hosts = hosts
                                self.dismiss()
                            }
                        } else {
                            DispatchQueue.main.async {
                                self.app.status = .error
                            }
                        }
                    }
                } else if type == "error" {
                    DispatchQueue.main.async {
                        self.app.status = .error
                    }
                } else {
                    DispatchQueue.main.async {
                        self.app.status = .error
                    }
                }
                task.cancel()

            case .failure(let error):
                DispatchQueue.main.async {
                    self.app.status = .error
                    print("[Connection] WebSocket receive error: \(error.localizedDescription)")
                }
                task.cancel()
            }
        }
    }
}
