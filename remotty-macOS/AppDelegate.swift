import AppKit

// MARK: - AppDelegate

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem!
    private var host: HostManager!
    private var menuRefreshTimer: Timer?

    private var settingsWC: NSWindowController?

    func applicationDidFinishLaunching(_ notification: Notification) {
        // MUST set activation policy FIRST
        NSApp.setActivationPolicy(.accessory)

        // Initialize on main thread via applicationDidFinishLaunching
        host = HostManager()

        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        buildMenu()
        updateIcon()

        // Auto-start signal server then host daemon
        host.startSignal()

        // Periodically refresh menu to update session list
        startMenuRefresh()
    }

    // MARK: - Menu Refresh

    private func startMenuRefresh() {
        menuRefreshTimer?.invalidate()
        menuRefreshTimer = Timer.scheduledTimer(withTimeInterval: 3, repeats: true) { [weak self] _ in
            guard let self = self else { return }
            // Only rebuild if sessions or running state changed
            self.buildMenu()
            self.updateIcon()
        }
    }

    // MARK: - Icon

    private func updateIcon() {
        guard let b = statusItem?.button else { return }
        let name = host.isRunning ? "terminal.fill" : "terminal"
        let img = NSImage(systemSymbolName: name, accessibilityDescription: "remotty")
        let config = NSImage.SymbolConfiguration(pointSize: 14, weight: .medium)
        b.image = img?.withSymbolConfiguration(config)
    }

    // MARK: - Menu

    private func buildMenu() {
        let menu = NSMenu()
        menu.autoenablesItems = false

        // Status header
        let si = NSMenuItem()
        let attrs: [NSAttributedString.Key: Any] = [
            .foregroundColor: host.isRunning ? NSColor.systemGreen : NSColor.secondaryLabelColor,
            .font: NSFont.monospacedSystemFont(ofSize: 12, weight: .semibold),
        ]
        si.attributedTitle = NSAttributedString(
            string: host.isRunning ? "●  Running" : "○  Stopped", attributes: attrs)
        si.isEnabled = false
        menu.addItem(si)

        // Sessions list
        if host.isRunning {
            if host.sessions.isEmpty {
                let noSess = NSMenuItem(title: "No connected devices",
                                       action: nil, keyEquivalent: "")
                noSess.isEnabled = false
                noSess.attributedTitle = NSAttributedString(
                    string: "No connected devices",
                    attributes: [.foregroundColor: NSColor.tertiaryLabelColor,
                                .font: NSFont.systemFont(ofSize: 12)])
                menu.addItem(noSess)
            } else {
                // Session header
                let sessHeader = NSMenuItem(title: "Connected Devices",
                                           action: nil, keyEquivalent: "")
                sessHeader.isEnabled = false
                let headerAttrs: [NSAttributedString.Key: Any] = [
                    .foregroundColor: NSColor.secondaryLabelColor,
                    .font: NSFont.systemFont(ofSize: 11, weight: .semibold),
                ]
                sessHeader.attributedTitle = NSAttributedString(
                    string: "CONNECTED DEVICES", attributes: headerAttrs)
                menu.addItem(sessHeader)

                for session in host.sessions {
                    let sessItem = NSMenuItem()
                    sessItem.isEnabled = false

                    let statusColor: NSColor = session.authed ? NSColor.systemGreen : NSColor.systemOrange
                    let deviceText = session.displayName
                    let durationText = session.duration
                    let statusIcon = session.authed ? "✓" : "⋯"

                    let paragraph = NSMutableParagraphStyle()
                    paragraph.lineSpacing = 2

                    let sessAttrs: [NSAttributedString.Key: Any] = [
                        .font: NSFont.systemFont(ofSize: 12),
                        .foregroundColor: NSColor.labelColor,
                        .paragraphStyle: paragraph,
                    ]
                    let detailAttrs: [NSAttributedString.Key: Any] = [
                        .font: NSFont.systemFont(ofSize: 10),
                        .foregroundColor: NSColor.secondaryLabelColor,
                        .paragraphStyle: paragraph,
                    ]

                    let str = NSMutableAttributedString()
                    str.append(NSAttributedString(string: "\(statusIcon) ", attributes: [
                        .font: NSFont.monospacedSystemFont(ofSize: 11, weight: .medium),
                        .foregroundColor: statusColor,
                    ]))
                    str.append(NSAttributedString(string: deviceText, attributes: sessAttrs))
                    str.append(NSAttributedString(string: "\n", attributes: sessAttrs))
                    str.append(NSAttributedString(string: "   \(durationText)  ·  \(session.statusLabel)", attributes: detailAttrs))

                    sessItem.attributedTitle = str
                    menu.addItem(sessItem)
                }
            }

            menu.addItem(.separator())
        }

        // Start / Stop
        let ti = NSMenuItem(title: host.isRunning ? "Stop Host" : "Start Host",
                          action: #selector(toggleHost), keyEquivalent: "h")
        ti.keyEquivalentModifierMask = [.command, .control]
        menu.addItem(ti)

        menu.addItem(.separator())

        // Web UI
        let wi = NSMenuItem(title: "Open Web UI",
                          action: #selector(openWebUI), keyEquivalent: "w")
        wi.keyEquivalentModifierMask = [.command, .control]
        wi.isEnabled = host.isRunning
        menu.addItem(wi)

        // Screen sharing
        let si2 = NSMenuItem(title: host.screenSharing ? "Stop Screen Sharing" : "Start Screen Sharing",
                           action: #selector(toggleScreenShare), keyEquivalent: "s")
        si2.keyEquivalentModifierMask = [.command, .control]
        si2.isEnabled = host.isRunning
        menu.addItem(si2)

        menu.addItem(.separator())

        // Settings
        let seti = NSMenuItem(title: "Settings…", action: #selector(openSettings), keyEquivalent: ",")
        menu.addItem(seti)

        // Quit
        let qi = NSMenuItem(title: "Quit remotty", action: #selector(quitApp), keyEquivalent: "q")
        menu.addItem(qi)

        statusItem.menu = menu
    }

    // MARK: - Actions

    @objc private func toggleHost() {
        if host.isRunning { host.stopHost() }
        else { host.startHost() }
        buildMenu()
        updateIcon()
    }

    @objc private func openWebUI() {
        guard let url = URL(string: "http://localhost:9000") else { return }
        NSWorkspace.shared.open(url)
    }

    @objc private func toggleScreenShare() {
        host.toggleScreenShare()
        buildMenu()
    }

    @objc private func openSettings() {
        if let wc = settingsWC {
            wc.window?.makeKeyAndOrderFront(nil)
            return
        }

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 380, height: 260),
            styleMask: [.titled, .closable, .miniaturizable],
            backing: .buffered, defer: false)
        window.title = "remotty Settings"
        window.center()

        let content = NSView(frame: NSRect(x: 0, y: 0, width: 380, height: 260))

        // Signal URL
        let urlLabel = NSTextField(labelWithString: "Signal URL:")
        urlLabel.frame = NSRect(x: 20, y: 220, width: 80, height: 20)
        urlLabel.font = NSFont.systemFont(ofSize: 12)

        let urlField = NSTextField(frame: NSRect(x: 110, y: 218, width: 250, height: 22))
        urlField.stringValue = host.signalURL
        urlField.placeholderString = "ws://localhost:9000"
        urlField.font = NSFont.monospacedSystemFont(ofSize: 12, weight: .regular)
        urlField.target = self
        urlField.action = #selector(urlChanged(_:))

        // Hostname
        let nameLabel = NSTextField(labelWithString: "Hostname:")
        nameLabel.frame = NSRect(x: 20, y: 185, width: 80, height: 20)
        nameLabel.font = NSFont.systemFont(ofSize: 12)

        let nameField = NSTextField(frame: NSRect(x: 110, y: 183, width: 250, height: 22))
        nameField.stringValue = host.hostName
        nameField.placeholderString = ProcessInfo.processInfo.hostName
        nameField.font = NSFont.monospacedSystemFont(ofSize: 12, weight: .regular)
        nameField.target = self
        nameField.action = #selector(nameChanged(_:))

        // Launch at login
        let loginCheck = NSButton(checkboxWithTitle: "Launch at login", target: self,
                                  action: #selector(loginToggled(_:)))
        loginCheck.frame = NSRect(x: 20, y: 145, width: 200, height: 20)
        loginCheck.state = host.launchAtLogin ? .on : .off

        // Separator
        let sep = NSBox(frame: NSRect(x: 20, y: 125, width: 340, height: 1))
        sep.boxType = .separator

        // Version
        let versionLabel = NSTextField(labelWithString: "remotty 0.7.1")
        versionLabel.frame = NSRect(x: 20, y: 95, width: 200, height: 20)
        versionLabel.font = NSFont.monospacedSystemFont(ofSize: 11, weight: .medium)
        versionLabel.textColor = .secondaryLabelColor

        let descLabel = NSTextField(labelWithString: "Remote terminal & screen access via WebRTC")
        descLabel.frame = NSRect(x: 20, y: 75, width: 340, height: 16)
        descLabel.font = NSFont.systemFont(ofSize: 10)
        descLabel.textColor = .tertiaryLabelColor

        content.addSubview(urlLabel)
        content.addSubview(urlField)
        content.addSubview(nameLabel)
        content.addSubview(nameField)
        content.addSubview(loginCheck)
        content.addSubview(sep)
        content.addSubview(versionLabel)
        content.addSubview(descLabel)

        window.contentView = content
        window.delegate = self

        let wc = NSWindowController(window: window)
        self.settingsWC = wc
        wc.showWindow(nil)
    }

    @objc private func quitApp() {
        host.stopAll()
        NSApp.terminate(nil)
    }

    // MARK: - Settings Actions

    @objc private func urlChanged(_ sender: NSTextField) {
        host.signalURL = sender.stringValue
    }

    @objc private func nameChanged(_ sender: NSTextField) {
        host.hostName = sender.stringValue
    }

    @objc private func loginToggled(_ sender: NSButton) {
        host.launchAtLogin = sender.state == .on
    }
}

// MARK: - NSWindowDelegate

extension AppDelegate: NSWindowDelegate {
    func windowWillClose(_ notification: Notification) {
        settingsWC = nil
    }
}
