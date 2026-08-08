import XCTest
@testable import remotty_ios

// MARK: - WebRTCService Tests

final class WebRTCServiceTests: XCTestCase {

    var service: WebRTCService!

    override func setUp() {
        super.setUp()
        service = WebRTCService()
    }

    override func tearDown() {
        service = nil
        super.tearDown()
    }

    // MARK: - Initial State

    func test_initialState_isDisconnected() {
        XCTAssertEqual(service.state, .disconnected)
    }

    func test_initialState_noCurrentFrame() {
        XCTAssertNil(service.currentFrame)
    }

    func test_initialState_defaultRemoteScreenSize() {
        XCTAssertEqual(service.remoteScreenSize, CGSize(width: 1920, height: 1080))
    }

    func test_initialState_terminalOutputIsEmpty() {
        XCTAssertTrue(service.terminalOutput.isEmpty)
    }

    // MARK: - DataChannelLabel Enum

    func test_dataChannelLabel_allCasesCount() {
        XCTAssertEqual(WebRTCService.DataChannelLabel.allCases.count, 4)
    }

    func test_dataChannelLabel_containsAllLabels() {
        let labels = WebRTCService.DataChannelLabel.allCases
        XCTAssertTrue(labels.contains(.terminal))
        XCTAssertTrue(labels.contains(.screen))
        XCTAssertTrue(labels.contains(.auth))
        XCTAssertTrue(labels.contains(.file))
    }

    func test_dataChannelLabel_rawValues() {
        XCTAssertEqual(WebRTCService.DataChannelLabel.terminal.rawValue, "terminal")
        XCTAssertEqual(WebRTCService.DataChannelLabel.screen.rawValue, "screen")
        XCTAssertEqual(WebRTCService.DataChannelLabel.auth.rawValue, "auth")
        XCTAssertEqual(WebRTCService.DataChannelLabel.file.rawValue, "file")
    }

    // MARK: - WebRTCState Equatable

    func test_webRTCState_equalDisconnected() {
        XCTAssertEqual(WebRTCState.disconnected, WebRTCState.disconnected)
    }

    func test_webRTCState_equalConnecting() {
        XCTAssertEqual(WebRTCState.connecting, WebRTCState.connecting)
    }

    func test_webRTCState_equalConnected() {
        XCTAssertEqual(WebRTCState.connected, WebRTCState.connected)
    }

    func test_webRTCState_equalError() {
        XCTAssertEqual(WebRTCState.error("test"), WebRTCState.error("test"))
    }

    func test_webRTCState_notEqualDifferentErrors() {
        XCTAssertNotEqual(WebRTCState.error("msg1"), WebRTCState.error("msg2"))
    }

    func test_webRTCState_notEqualDifferentStates() {
        XCTAssertNotEqual(WebRTCState.connected, WebRTCState.disconnected)
        XCTAssertNotEqual(WebRTCState.connecting, .error("x"))
    }

    // MARK: - Connect (Invalid URL)

    func test_connect_withEmptyString_setsError() {
        let exp = expectation(description: "Wait for error state")
        service.connect(signalURL: "", hostID: "test-host")

        DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
            if case .error(let msg) = self.service.state {
                XCTAssertFalse(msg.isEmpty)
            } else {
                XCTFail("Expected .error state after connect with empty URL, got \(self.service.state)")
            }
            exp.fulfill()
        }
        wait(for: [exp], timeout: 1.0)
    }

    func test_connect_withInvalidFormat_setsError() {
        let exp = expectation(description: "Wait for error state")
        let invalidURL = "http://"
        service.connect(signalURL: invalidURL, hostID: "test-host")

        DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
            if case .error = self.service.state {
                // Expected: URL parsing fails
            } else {
                XCTFail("Expected .error state, got \(self.service.state)")
            }
            exp.fulfill()
        }
        wait(for: [exp], timeout: 1.0)
    }

    // MARK: - Disconnect

    func test_disconnect_setsStateToDisconnected() {
        service.disconnect()
        XCTAssertEqual(service.state, .disconnected)
    }

    func test_disconnect_calledMultipleTimes_noCrash() {
        service.disconnect()
        service.disconnect()
        service.disconnect()
        XCTAssertEqual(service.state, .disconnected)
    }

    // MARK: - Send Methods — No-Crash Tests (when not connected)

    func test_sendData_whenDisconnected_doesNotCrash() {
        let data = "hello".data(using: .utf8)!
        service.send(data: data, channel: .terminal)
    }

    func test_sendJSON_whenDisconnected_doesNotCrash() {
        service.send(json: ["key": "value"], channel: .screen)
    }

    func test_sendProtocol_whenDisconnected_doesNotCrash() {
        service.sendProtocol(type: "test", payload: ["foo": "bar"], channel: .auth)
    }

    func test_sendProtocol_withEmptyPayload_doesNotCrash() {
        service.sendProtocol(type: "ping", channel: .screen)
    }

    func test_sendMouseMove_whenDisconnected_doesNotCrash() {
        service.sendMouseMove(x: 100.0, y: 200.0)
    }

    func test_sendMouseClick_whenDisconnected_doesNotCrash() {
        service.sendMouseClick(button: 0, x: 100.0, y: 200.0, down: true)
    }

    func test_sendScroll_whenDisconnected_doesNotCrash() {
        service.sendScroll(deltaX: 10.0, deltaY: 20.0)
    }

    func test_sendKey_withChars_whenDisconnected_doesNotCrash() {
        service.sendKey(keyCode: 36, chars: "a", down: true)
    }

    func test_sendKey_withoutChars_whenDisconnected_doesNotCrash() {
        service.sendKey(keyCode: 36, down: true)
    }

    func test_sendKey_keyRelease_whenDisconnected_doesNotCrash() {
        service.sendKey(keyCode: 36, down: false)
    }

    func test_sendAuth_whenDisconnected_doesNotCrash() {
        service.sendAuth(password: "secret123")
    }

    func test_sendTerminalInput_whenDisconnected_doesNotCrash() {
        service.sendTerminalInput("ls -la /tmp")
    }

    func test_sendTerminalInput_withEmptyString_doesNotCrash() {
        service.sendTerminalInput("")
    }

    func test_sendTerminalResize_whenDisconnected_doesNotCrash() {
        service.sendTerminalResize(rows: 24, cols: 80)
    }

    func test_sendTerminalResize_customDimensions_doesNotCrash() {
        service.sendTerminalResize(rows: 40, cols: 120)
    }

    func test_requestScreenStart_withCustomParams_doesNotCrash() {
        service.requestScreenStart(fps: 30, quality: 85)
    }

    func test_requestScreenStart_withDefaults_doesNotCrash() {
        service.requestScreenStart()
    }

    func test_requestScreenStop_whenDisconnected_doesNotCrash() {
        service.requestScreenStop()
    }

    // MARK: - Delegate (Weak Reference)

    func test_delegate_isHeldWeakly() {
        class MockDelegate: WebRTCServiceDelegate {
            func webRTCService(_: WebRTCService, didReceiveFrame _: UIImage) {}
            func webRTCService(_: WebRTCService, didChangeState _: WebRTCState) {}
            func webRTCService(_: WebRTCService, didReceiveTerminalOutput _: String) {}
            func webRTCService(_: WebRTCService, didReceiveError _: Error) {}
        }

        var delegate: MockDelegate? = MockDelegate()
        service.delegate = delegate
        XCTAssertNotNil(service.delegate)

        delegate = nil
        XCTAssertNil(service.delegate, "Delegate should be nil after strong reference is dropped (weak reference)")
    }

    func test_delegate_canBeSetToNil() {
        service.delegate = nil
        XCTAssertNil(service.delegate)
    }

    // MARK: - Observable Object Conformance

    func test_service_isObservableObject() {
        // Compile-time check: WebRTCService conforms to ObservableObject
        let _: any ObservableObject = service
    }

    // MARK: - Type Validation (Compile-Time Checks)

    func test_dataChannelLabel_isStringRawRepresentable() {
        // Compile-time check: RawRepresentable conformance
        let _ = WebRTCService.DataChannelLabel(rawValue: "terminal")
        XCTAssertNotNil(WebRTCService.DataChannelLabel(rawValue: "terminal"))
        XCTAssertNil(WebRTCService.DataChannelLabel(rawValue: "unknown"))
    }
}
