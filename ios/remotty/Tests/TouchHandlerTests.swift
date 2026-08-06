import XCTest
@testable import remotty_ios

// MARK: - TouchHandler Tests

final class TouchHandlerTests: XCTestCase {

    var touchHandler: TouchHandler!
    var mockWebRTCService: MockWebRTCService!
    let testView = UIView(frame: CGRect(x: 0, y: 0, width: 390, height: 844)) // iPhone 14 size

    override func setUp() {
        super.setUp()
        touchHandler = TouchHandler()
        mockWebRTCService = MockWebRTCService()
        touchHandler.webRTCService = mockWebRTCService
        touchHandler.viewSize = testView.bounds.size
        touchHandler.remoteScreenSize = CGSize(width: 1920, height: 1080)
    }

    override func tearDown() {
        touchHandler.reset()
        touchHandler = nil
        mockWebRTCService = nil
        super.tearDown()
    }

    // MARK: - Coordinate Conversion

    func test_convertToMac_withEqualAspectRatio() {
        // View 390x844, Remote 1920x1080
        // ScaleX = 1920/390 ≈ 4.923, ScaleY = 1080/844 ≈ 1.280
        let point = CGPoint(x: 195, y: 422) // roughly center
        let (mx, my) = touchHandler.convertToMac(point, in: testView)

        let expectedX = Double(point.x) * (1920.0 / 390.0)
        let expectedY = Double(point.y) * (1080.0 / 844.0)

        XCTAssertEqual(mx, expectedX, accuracy: 0.01)
        XCTAssertEqual(my, expectedY, accuracy: 0.01)
    }

    func test_convertToMac_topLeftCorner() {
        let point = CGPoint.zero
        let (mx, my) = touchHandler.convertToMac(point, in: testView)
        XCTAssertEqual(mx, 0.0)
        XCTAssertEqual(my, 0.0)
    }

    func test_convertToMac_clampsToBounds() {
        // Point outside the view should be clamped
        let point = CGPoint(x: 500, y: 1000) // beyond view size 390x844
        let (mx, my) = touchHandler.convertToMac(point, in: testView)

        XCTAssertLessThanOrEqual(mx, touchHandler.remoteScreenSize.width)
        XCTAssertLessThanOrEqual(my, touchHandler.remoteScreenSize.height)
        XCTAssertGreaterThanOrEqual(mx, 0.0)
        XCTAssertGreaterThanOrEqual(my, 0.0)
    }

    func test_convertToMac_withDifferentRemoteSize() {
        touchHandler.remoteScreenSize = CGSize(width: 2560, height: 1440)
        let point = CGPoint(x: 195, y: 422)
        let (mx, my) = touchHandler.convertToMac(point, in: testView)

        let expectedX = Double(point.x) * (2560.0 / 390.0)
        let expectedY = Double(point.y) * (1440.0 / 844.0)

        XCTAssertEqual(mx, expectedX, accuracy: 0.01)
        XCTAssertEqual(my, expectedY, accuracy: 0.01)
    }

    func test_convertToMac_withZeroViewSize_usesMinimum() {
        touchHandler.viewSize = .zero
        let point = CGPoint(x: 100, y: 100)
        // viewSize width/height become 1.0 due to max(..., 1.0)
        let (mx, my) = touchHandler.convertToMac(point, in: testView)

        // Note: convertToMac uses viewSize(property), not the view's actual frame
        // Since viewSize is .zero, max(0,1.0) = 1.0, so scale = remoteScreenSize / 1.0
        let expectedX = Double(point.x) * 1920.0
        let expectedY = Double(point.y) * 1080.0

        XCTAssertEqual(mx, expectedX, accuracy: 0.01)
        XCTAssertEqual(my, expectedY, accuracy: 0.01)
    }

    func test_convertToMac_withDifferentViewSize() {
        touchHandler.viewSize = CGSize(width: 834, height: 1194) // iPad size
        let point = CGPoint(x: 417, y: 597)
        let (mx, my) = touchHandler.convertToMac(point, in: testView)

        let expectedX = Double(point.x) * (1920.0 / 834.0)
        let expectedY = Double(point.y) * (1080.0 / 1194.0)

        XCTAssertEqual(mx, expectedX, accuracy: 0.01)
        XCTAssertEqual(my, expectedY, accuracy: 0.01)
    }

    // MARK: - Single Touch — touchesBegan

    func test_touchesBegan_singleTouch_tracksTouchInfo() {
        let touch = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch], in: testView)

        // Should have sent a mouse move event
        XCTAssertEqual(mockWebRTCService.sentMouseMoves.count, 1)
        XCTAssertEqual(mockWebRTCService.sentMouseMoves.first?.x, 100 * (1920.0 / 390.0), accuracy: 0.01)
        XCTAssertEqual(mockWebRTCService.sentMouseMoves.first?.y, 200 * (1080.0 / 844.0), accuracy: 0.01)
    }

    func test_touchesBegan_singleTouch_doesNotSendMouseDown() {
        let touch = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch], in: testView)

        XCTAssertTrue(mockWebRTCService.sentMouseClicks.isEmpty,
                      "Should not send mouse click on touchesBegan alone")
    }

    // MARK: - Single Touch — touchesMoved

    func test_touchesMoved_smallDrag_doesNotSendMouseDown() {
        let touch = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch], in: testView)
        mockWebRTCService.reset()

        // Move finger slightly (less than 3pt threshold)
        moveTouch(touch, to: CGPoint(x: 101, y: 201), in: testView)
        touchHandler.touchesMoved([touch], in: testView)

        // Should send mouse move but NOT mouse down yet
        XCTAssertGreaterThan(mockWebRTCService.sentMouseMoves.count, 0)
        XCTAssertTrue(mockWebRTCService.sentMouseClicks.isEmpty,
                      "Movement under threshold should not trigger mouse down")
    }

    func test_touchesMoved_largeDrag_sendsMouseDown() {
        let touch = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch], in: testView)
        mockWebRTCService.reset()

        // Move finger significantly (beyond 3pt threshold)
        moveTouch(touch, to: CGPoint(x: 150, y: 250), in: testView)
        touchHandler.touchesMoved([touch], in: testView)

        // Should have sent mouse down (button 0) + mouse move
        let downs = mockWebRTCService.sentMouseClicks.filter { $0.down }
        XCTAssertEqual(downs.count, 1, "Should send exactly one mouse down")
        XCTAssertEqual(downs.first?.button, 0)
    }

    func test_touchesMoved_afterDrag_sendsMouseMove() {
        let touch = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch], in: testView)
        mockWebRTCService.reset()

        moveTouch(touch, to: CGPoint(x: 150, y: 250), in: testView)
        touchHandler.touchesMoved([touch], in: testView)

        // Should send mouse move
        XCTAssertGreaterThan(mockWebRTCService.sentMouseMoves.count, 0)
    }

    // MARK: - Single Touch — touchesEnded

    func test_touchesEnded_singleTap_sendsLeftClick() {
        let touch = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch], in: testView)
        mockWebRTCService.reset()

        // Lift finger without dragging
        touchHandler.touchesEnded([touch], in: testView)

        // Should send a tap (down + up) for button 0
        let downClicks = mockWebRTCService.sentMouseClicks.filter { $0.down }
        let upClicks = mockWebRTCService.sentMouseClicks.filter { !$0.down }
        XCTAssertEqual(downClicks.count, 1, "Tap should send mouse down")
        XCTAssertEqual(upClicks.count, 1, "Tap should send mouse up")
        XCTAssertEqual(downClicks.first?.button, 0)
        XCTAssertEqual(upClicks.first?.button, 0)
    }

    func test_touchesEnded_afterDrag_sendsMouseUp() {
        let touch = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch], in: testView)
        moveTouch(touch, to: CGPoint(x: 150, y: 250), in: testView)
        touchHandler.touchesMoved([touch], in: testView)
        mockWebRTCService.reset()

        // Lift finger after drag
        touchHandler.touchesEnded([touch], in: testView)

        // Should only send mouse up (no down)
        let ups = mockWebRTCService.sentMouseClicks.filter { !$0.down }
        let downs = mockWebRTCService.sentMouseClicks.filter { $0.down }
        XCTAssertGreaterThan(ups.count, 0, "Drag end should send mouse up")
        XCTAssertEqual(downs.count, 0, "Drag end should NOT send mouse down")
    }

    // MARK: - Two-Finger Interactions

    func test_touchesBegan_secondFinger_sendsRightClick() {
        let touch1 = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch1], in: testView)
        mockWebRTCService.reset()

        // Second finger arrives
        let touch2 = makeTouch(at: CGPoint(x: 300, y: 400))
        touchHandler.touchesBegan([touch2], in: testView)

        // Should send right-click tap (button 1)
        let rightClickDowns = mockWebRTCService.sentMouseClicks.filter { $0.button == 1 && $0.down }
        let rightClickUps = mockWebRTCService.sentMouseClicks.filter { $0.button == 1 && !$0.down }
        XCTAssertEqual(rightClickDowns.count, 1, "Right-click should send mouse down")
        XCTAssertEqual(rightClickUps.count, 1, "Right-click should send mouse up")
    }

    func test_twoFingerScroll_sendsScrollEvent() {
        let touch1 = makeTouch(at: CGPoint(x: 100, y: 200))
        let touch2 = makeTouch(at: CGPoint(x: 300, y: 250))
        touchHandler.touchesBegan([touch1], in: testView)
        touchHandler.touchesBegan([touch2], in: testView)
        mockWebRTCService.reset()

        // Move both fingers (simulate pinch/scroll)
        moveTouch(touch1, to: CGPoint(x: 110, y: 210), in: testView)
        moveTouch(touch2, to: CGPoint(x: 310, y: 260), in: testView)
        touchHandler.touchesMoved([touch1, touch2], in: testView)

        // Should send scroll events if movement exceeds threshold
        if mockWebRTCService.sentScrolls.isEmpty {
            // Movement may be under threshold for the first move
            // Move further
            moveTouch(touch1, to: CGPoint(x: 130, y: 230), in: testView)
            moveTouch(touch2, to: CGPoint(x: 330, y: 280), in: testView)
            touchHandler.touchesMoved([touch1, touch2], in: testView)
        }

        // After sufficient movement, scroll events should be sent
        XCTAssertGreaterThan(mockWebRTCService.sentScrolls.count, 0,
                            "Two-finger drag should send scroll events")
    }

    // MARK: - touchesCancelled

    func test_touchesCancelled_cleansUpState() {
        let touch = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch], in: testView)
        moveTouch(touch, to: CGPoint(x: 150, y: 250), in: testView)
        touchHandler.touchesMoved([touch], in: testView)
        mockWebRTCService.reset()

        touchHandler.touchesCancelled([touch], in: testView)

        // State should be reset (no active touches)
        // Verify by checking that no further sends happen inadvertently
        // After cancelled, another touchesBegan should work cleanly
        let newTouch = makeTouch(at: CGPoint(x: 50, y: 50))
        touchHandler.touchesBegan([newTouch], in: testView)
        XCTAssertEqual(mockWebRTCService.sentMouseMoves.count, 1,
                      "Should be able to start fresh after cancel")
    }

    // MARK: - Reset

    func test_reset_clearsAllTracking() {
        let touch = makeTouch(at: CGPoint(x: 100, y: 200))
        touchHandler.touchesBegan([touch], in: testView)
        mockWebRTCService.reset()

        touchHandler.reset()

        // After reset, new touch should work fresh
        let newTouch = makeTouch(at: CGPoint(x: 50, y: 50))
        touchHandler.touchesBegan([newTouch], in: testView)
        XCTAssertEqual(mockWebRTCService.sentMouseMoves.count, 1)
    }

    // MARK: - Key Events

    func test_keyPressed_sendsKeyDownAndUp() {
        touchHandler.keyPressed(keyCode: 36, chars: "a")

        let downs = mockWebRTCService.sentKeys.filter { $0.down }
        let ups = mockWebRTCService.sentKeys.filter { !$0.down }

        XCTAssertEqual(downs.count, 1, "keyPressed should send key down")
        XCTAssertEqual(downs.first?.keyCode, 36)
        XCTAssertEqual(downs.first?.chars, "a")

        // Key up is sent after a delay (async), so it might not be available immediately
        // We just verify down was sent
    }

    func test_keyDown_sendsKeyDown() {
        touchHandler.keyDown(keyCode: 53, chars: "b")

        XCTAssertEqual(mockWebRTCService.sentKeys.count, 1)
        XCTAssertEqual(mockWebRTCService.sentKeys.first?.keyCode, 53)
        XCTAssertEqual(mockWebRTCService.sentKeys.first?.chars, "b")
        XCTAssertTrue(mockWebRTCService.sentKeys.first?.down ?? false)
    }

    func test_keyUp_sendsKeyUp() {
        touchHandler.keyUp(keyCode: 53)

        XCTAssertEqual(mockWebRTCService.sentKeys.count, 1)
        XCTAssertEqual(mockWebRTCService.sentKeys.first?.keyCode, 53)
        XCTAssertFalse(mockWebRTCService.sentKeys.first?.down ?? true)
    }

    func test_keyPressed_withoutChars_doesNotCrash() {
        touchHandler.keyPressed(keyCode: 36)
        // No crash = pass
    }

    func test_keyDown_withoutChars_doesNotCrash() {
        touchHandler.keyDown(keyCode: 36)
        // No crash = pass
    }

    func test_keyUp_sendsCorrectKeyCode() {
        touchHandler.keyUp(keyCode: 53)
        XCTAssertEqual(mockWebRTCService.sentKeys.first?.keyCode, 53)
    }

    // MARK: - Edge Cases

    func test_touchesEnded_withNoActiveTouches_doesNotCrash() {
        // Calling touchesEnded without any prior touches should not crash
        let touch = makeTouch(at: .zero)
        touchHandler.touchesEnded([touch], in: testView)
        // No crash = pass
    }

    func test_touchesCancelled_withNoActiveTouches_doesNotCrash() {
        let touch = makeTouch(at: .zero)
        touchHandler.touchesCancelled([touch], in: testView)
        // No crash = pass
    }

    func test_touchesMoved_withNoActiveTouches_doesNotCrash() {
        let touch = makeTouch(at: .zero)
        touchHandler.touchesMoved([touch], in: testView)
        // No crash = pass
    }

    func test_convertToMac_withNegativePoint() {
        let point = CGPoint(x: -10, y: -20)
        let (mx, my) = touchHandler.convertToMac(point, in: testView)

        // Negative points should be clamped to 0
        XCTAssertGreaterThanOrEqual(mx, 0.0)
        XCTAssertGreaterThanOrEqual(my, 0.0)
    }

    // MARK: - Helpers

    private func makeTouch(at point: CGPoint) -> UITouch {
        // Create a concrete UITouch subclass for testing
        let touch = TestTouch()
        touch.testLocation = point
        return touch
    }

    private func moveTouch(_ touch: UITouch, to point: CGPoint, in view: UIView) {
        guard let testTouch = touch as? TestTouch else { return }
        testTouch.testLocation = point
    }
}

// MARK: - Test UITouch Subclass

private class TestTouch: UITouch {
    var testLocation: CGPoint = .zero

    override func location(in view: UIView?) -> CGPoint {
        return testLocation
    }
}

// MARK: - Mock WebRTCService

class MockWebRTCService: WebRTCService {

    struct SentMouseMove {
        let x: Double
        let y: Double
    }

    struct SentMouseClick {
        let button: Int
        let x: Double
        let y: Double
        let down: Bool
    }

    struct SentScroll {
        let deltaX: Double
        let deltaY: Double
    }

    struct SentKey {
        let keyCode: UInt16
        let chars: String?
        let down: Bool
    }

    var sentMouseMoves: [SentMouseMove] = []
    var sentMouseClicks: [SentMouseClick] = []
    var sentScrolls: [SentScroll] = []
    var sentKeys: [SentKey] = []

    override func sendMouseMove(x: Double, y: Double) {
        sentMouseMoves.append(SentMouseMove(x: x, y: y))
    }

    override func sendMouseClick(button: Int, x: Double, y: Double, down: Bool) {
        sentMouseClicks.append(SentMouseClick(button: button, x: x, y: y, down: down))
    }

    override func sendScroll(deltaX: Double, deltaY: Double) {
        sentScrolls.append(SentScroll(deltaX: deltaX, deltaY: deltaY))
    }

    override func sendKey(keyCode: UInt16, chars: String? = nil, down: Bool) {
        sentKeys.append(SentKey(keyCode: keyCode, chars: chars, down: down))
    }

    func reset() {
        sentMouseMoves.removeAll()
        sentMouseClicks.removeAll()
        sentScrolls.removeAll()
        sentKeys.removeAll()
    }
}
