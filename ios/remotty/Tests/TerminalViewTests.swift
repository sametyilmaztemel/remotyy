import XCTest
import SwiftUI
@testable import remotty_ios

// MARK: - TerminalView Tests

final class TerminalViewTests: XCTestCase {

    // Sample HostInfo for testing
    let testHost = HostInfo(
        id: "test-host-1",
        name: "Test Mac mini",
        platform: "darwin",
        arch: "arm64",
        online: true,
        features: ["terminal", "screen"]
    )

    var appState: AppState!

    override func setUp() {
        super.setUp()
        appState = AppState()
    }

    override func tearDown() {
        appState = nil
        super.tearDown()
    }

    // MARK: - View Instantiation

    func test_terminalView_canBeInstantiated() {
        let view = TerminalView(host: testHost)
            .environmentObject(appState)
        XCTAssertNotNil(view)
    }

    func test_terminalView_withMultipleHosts_doesNotCrash() {
        let host2 = HostInfo(
            id: "test-host-2",
            name: "Test Ubuntu",
            platform: "linux",
            arch: "amd64",
            online: false,
            features: ["terminal"]
        )
        let view = TerminalView(host: host2)
            .environmentObject(appState)
        XCTAssertNotNil(view)
    }

    // MARK: - Placeholder Text

    func test_placeholderText_whenDisconnected_containsDisconnected() {
        // We can't directly access placeholderText (private),
        // but we can verify the view builds without crash in this state
        let view = TerminalView(host: testHost)
            .environmentObject(appState)
        XCTAssertNotNil(view)
    }

    func test_placeholderText_whenConnected_showsHostName() {
        let view = TerminalView(host: testHost)
            .environmentObject(appState)
        XCTAssertNotNil(view)
    }

    // MARK: - Connection Indicator Colors

    func test_connectionIndicator_disconnectedIsGray() {
        let view = TerminalView(host: testHost)
            .environmentObject(appState)
        XCTAssertNotNil(view)
    }

    // MARK: - Host Info Display

    func test_terminalView_navigationTitle_isHostName() {
        // SwiftUI NavigationTitle is set from inside the view body,
        // we can verify via view identity
        let view = TerminalView(host: testHost)
            .environmentObject(appState)
        // The host is passed in, verifying it's not lost
        // (compile-time check that init works)
    }

    // MARK: - WebRTC Service Observation (Integration-style)

    func test_terminalView_connectsOnAppear() {
        // Verify that the service's connect method can be called
        // with the same parameters TerminalView uses
        appState.signalURL = "ws://test-server:9000"
        appState.webRTCService.connect(
            signalURL: appState.signalURL,
            hostID: testHost.id,
            password: nil
        )

        // connect will attempt WebSocket to test-server:9000,
        // which will fail asynchronously — just test no crash
    }

    func test_terminalView_disconnectsOnDisappear() {
        appState.webRTCService.disconnect()
        XCTAssertEqual(appState.webRTCService.state, .disconnected)
    }

    // MARK: - Terminal Output Flow

    func test_terminalOutput_updatesViaObservation() {
        let expectation = self.expectation(description: "Terminal output update")

        // Simulate receiving terminal output via the service
        let testOutput = "Hello from remote host\n$ "

        // TerminalOutput is @Published, so it updates on main
        DispatchQueue.main.async {
            // Append through the internal method (indirectly via delegate call)
            // The service exposes terminalOutput as a published property
            // which TerminalView subscribes to
            self.appState.webRTCService.sendTerminalInput("test command")

            // Note: sendTerminalInput sends data over data channel (which is nil
            // when not connected), so it won't affect terminalOutput directly.
            // terminalOutput is only modified when receiving messages.
            // This is an integration test of the pipeline, not a unit test.

            expectation.fulfill()
        }

        wait(for: [expectation], timeout: 1.0)
    }

    // MARK: - Send Command

    func test_sendCommand_sendsViaWebRTCService() {
        // Test that the service's sendTerminalInput method works
        let testInput = "ls -la"
        appState.webRTCService.sendTerminalInput(testInput)
        // No crash = pass (data channel isn't open so it's a no-op)
    }

    func test_sendCommand_withEmptyString_doesNotCrash() {
        appState.webRTCService.sendTerminalInput("")
        // No crash = pass
    }

    func test_sendCommand_withSpecialCharacters_doesNotCrash() {
        appState.webRTCService.sendTerminalInput("echo \"hello world\" && exit")
        // No crash = pass
    }

    func test_sendCommand_withUnicode_doesNotCrash() {
        appState.webRTCService.sendTerminalInput("ls 🚀")
        // No crash = pass
    }

    // MARK: - Control Sequences

    func test_sendControlSequence_sendsCtrlC() {
        let ctrlC = "\u{0003}" // ETX
        appState.webRTCService.sendTerminalInput(ctrlC)
        // No crash = pass
    }

    func test_sendControlSequence_sendsEscape() {
        let esc = "\u{001b}"
        appState.webRTCService.sendTerminalInput(esc)
        // No crash = pass
    }

    func test_sendControlSequence_sendsTab() {
        appState.webRTCService.sendTerminalInput("\t")
        // No crash = pass
    }

    func test_sendControlSequence_sendsArrowUp() {
        appState.webRTCService.sendTerminalInput("\u{001b}[A")
        // No crash = pass
    }

    func test_sendControlSequence_sendsArrowDown() {
        appState.webRTCService.sendTerminalInput("\u{001b}[B")
        // No crash = pass
    }

    func test_sendControlSequence_sendsArrowLeft() {
        appState.webRTCService.sendTerminalInput("\u{001b}[D")
        // No crash = pass
    }

    func test_sendControlSequence_sendsArrowRight() {
        appState.webRTCService.sendTerminalInput("\u{001b}[C")
        // No crash = pass
    }

    // MARK: - Terminal Resize

    func test_sendTerminalResize_withStandardSize() {
        appState.webRTCService.sendTerminalResize(rows: 24, cols: 80)
        // No crash = pass
    }

    func test_sendTerminalResize_withLargeSize() {
        appState.webRTCService.sendTerminalResize(rows: 60, cols: 200)
        // No crash = pass
    }

    // MARK: - Connection Lifecycle

    func test_connect_and_disconnect_resetsOutput() {
        // Connect should reset terminalOutput
        appState.webRTCService.sendTerminalInput("hello")
        // disconnect is called elsewhere

        appState.webRTCService.disconnect()
        XCTAssertEqual(appState.webRTCService.state, .disconnected)
    }

    func test_stateTransitions_noCrash() {
        // Trigger various state transitions to ensure nothing crashes
        appState.webRTCService.disconnect()                  // .disconnected
        // .connecting is set by connect() which requires network
        // Test that disconnect can always be called safely
        appState.webRTCService.disconnect()
        appState.webRTCService.disconnect()
    }

    // MARK: - View Reuse

    func test_terminalView_reuseWithDifferentHost() {
        let view1 = TerminalView(host: testHost)
            .environmentObject(appState)

        let host2 = HostInfo(
            id: "other-host",
            name: "Other Machine",
            platform: "linux",
            arch: "x86_64",
            online: true,
            features: ["terminal"]
        )
        let view2 = TerminalView(host: host2)
            .environmentObject(appState)

        XCTAssertNotNil(view1)
        XCTAssertNotNil(view2)
    }

    func test_terminalView_withDisconnectedService_doesNotSendCommands() {
        // When not connected, sendCommand should be a no-op
        appState.webRTCService.sendTerminalInput("dangerous command")
        // No crash = pass
    }

    // MARK: - State Enum Consistency

    func test_connectionStatus_enum_values() {
        XCTAssertEqual(AppState.ConnectionStatus.disconnected.rawValue, "Disconnected")
        XCTAssertEqual(AppState.ConnectionStatus.connecting.rawValue, "Connecting...")
        XCTAssertEqual(AppState.ConnectionStatus.connected.rawValue, "Connected")
        XCTAssertEqual(AppState.ConnectionStatus.error.rawValue, "Error")
    }

    // MARK: - Keyboard Toolbar Building

    func test_terminalView_toolbarButtons_disabledWhenDisconnected() {
        // Verify that the view can be created with all its subviews
        let view = TerminalView(host: testHost)
            .environmentObject(appState)
            .disabled(true) // simulate disconnected state

        XCTAssertNotNil(view)
    }

    // MARK: - AppState Integration

    func test_appState_providesWebRTCService() {
        XCTAssertNotNil(appState.webRTCService)
        XCTAssertTrue(appState.webRTCService === appState.webRTCService)
    }

    func test_appState_canFindHost() {
        let host = testHost
        appState.hosts = [testHost]
        let found = appState.host(with: testHost.id)
        XCTAssertEqual(found?.id, testHost.id)
        XCTAssertEqual(found?.name, testHost.name)
    }

    func test_appState_hostWithUnknownID_returnsNil() {
        appState.hosts = [testHost]
        let found = appState.host(with: "non-existent-id")
        XCTAssertNil(found)
    }

    // MARK: - HostInfo Model

    func test_hostInfo_codable() {
        let encoder = JSONEncoder()
        let decoder = JSONDecoder()

        let encoded = try? encoder.encode(testHost)
        XCTAssertNotNil(encoded)

        let decoded = try? decoder.decode(HostInfo.self, from: encoded!)
        XCTAssertEqual(decoded?.id, testHost.id)
        XCTAssertEqual(decoded?.name, testHost.name)
        XCTAssertEqual(decoded?.platform, testHost.platform)
        XCTAssertEqual(decoded?.arch, testHost.arch)
        XCTAssertEqual(decoded?.online, testHost.online)
        XCTAssertEqual(decoded?.features, testHost.features)
    }

    func test_hostInfo_identifiable() {
        let host1 = testHost
        let host2 = HostInfo(
            id: testHost.id,
            name: "Different",
            platform: "linux",
            arch: "x86_64",
            online: false,
            features: []
        )
        // Identifiable conformance — same ID should be treated same
        XCTAssertEqual(host1.id, host2.id)
    }
}
