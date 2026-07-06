import Foundation
import UIKit

// MARK: - Touch → macOS Input Translator

/// Converts iOS touch events into remote macOS input messages and sends them
/// over the WebRTC screen data channel.
///
/// Coordinate flow:
///   UITouch point (in view coordinates) → scaled to remote display → JSON message
final class TouchHandler {

    // MARK: Configuration

    /// The view's current size (updated by ScreenRendererView on layout).
    var viewSize: CGSize = .zero

    /// The remote display's pixel dimensions (updated from `screen_resize` messages).
    var remoteScreenSize: CGSize = CGSize(width: 1920, height: 1080)

    /// Weak reference to the WebRTC service for sending messages.
    weak var webRTCService: WebRTCService?

    // MARK: Tracking

    /// Active touches keyed by `UITouch` pointer address.
    private var activeTouchInfo: [ObjectIdentifier: TouchInfo] = [:]

    /// Whether we've sent a mouse-down without a corresponding mouse-up.
    private var mouseIsDown = false

    /// The last reported mouse position (to avoid redundant move events).
    private var lastReportedPosition: CGPoint?

    /// Gesture detection state.
    private enum GestureState {
        case none
        case possibleScroll(startPoint: CGPoint, startTime: TimeInterval)
        case scrolling
    }
    private var gestureState: GestureState = .none

    // MARK: Constants

    /// Minimum distance (in points) before a pan is treated as a scroll rather than a tap.
    private let scrollThreshold: CGFloat = 12.0

    /// Maximum time (seconds) for a touch to count as a tap.
    private let tapTimeThreshold: TimeInterval = 0.3

    // MARK: Touch lifecycle

    /// Call when `touchesBegan` is received on the render view.
    /// - touches: The set of new touches.
    /// - view:    The view the touches belong to.
    func touchesBegan(_ touches: Set<UITouch>, in view: UIView) {
        let now = CFAbsoluteTimeGetCurrent()
        for touch in touches {
            let key = ObjectIdentifier(touch)
            let location = touch.location(in: view)
            activeTouchInfo[key] = TouchInfo(
                startPoint: location,
                previousPoint: location,
                currentPoint: location,
                startTime: now
            )
        }

        let count = activeTouchInfo.count

        switch count {
        case 1:
            // Single touch: begin tracking for mouse or two-finger tap detection
            if let first = activeTouchInfo.first {
                gestureState = .possibleScroll(startPoint: first.value.startPoint, startTime: now)
            }
            // Don't send mouse_down yet — wait to distinguish tap from drag
            break

        case 2 where touches.count == 1:
            // Second finger just arrived while first is still down → right-click
            handleRightClick(in: view)

        default:
            break
        }

        // Immediately report position
        if let touch = touches.first {
            let point = touch.location(in: view)
            sendMouseMove(from: point, in: view)
        }
    }

    /// Call when `touchesMoved` is received.
    func touchesMoved(_ touches: Set<UITouch>, in view: UIView) {
        for touch in touches {
            let key = ObjectIdentifier(touch)
            let location = touch.location(in: view)
            activeTouchInfo[key]?.previousPoint = activeTouchInfo[key]?.currentPoint ?? location
            activeTouchInfo[key]?.currentPoint = location
        }

        let count = activeTouchInfo.count

        switch count {
        case 1:
            // Single-finger drag → mouse move (and mouse-down if not already)
            if !mouseIsDown, let first = activeTouchInfo.first {
                let traveled = hypot(
                    first.value.currentPoint.x - first.value.startPoint.x,
                    first.value.currentPoint.y - first.value.startPoint.y
                )
                // Only send mouse-down after a small movement to avoid accidental clicks
                if traveled > 3.0 {
                    let (mx, my) = convertToMac(first.value.currentPoint, in: view)
                    sendMouseDown(button: 0, x: mx, y: my)
                    mouseIsDown = true
                    gestureState = .none
                }
            }
            // Send mouse move
            if let first = activeTouchInfo.first {
                sendMouseMove(from: first.value.currentPoint, in: view)
            }

        case 2:
            // Two-finger gesture — check for scroll
            handleTwoFingerScroll(in: view)

        default:
            break
        }
    }

    /// Call when `touchesEnded` is received.
    func touchesEnded(_ touches: Set<UITouch>, in view: UIView) {
        let wasMultiTouch = activeTouchInfo.count > 1

        for touch in touches {
            let key = ObjectIdentifier(touch)
            activeTouchInfo.removeValue(forKey: key)
        }

        let remaining = activeTouchInfo.count

        switch remaining {
        case 0:
            // All fingers lifted
            if mouseIsDown {
                // If we had a drag, lift the mouse button
                sendMouseUp(button: 0)
                mouseIsDown = false
            } else if !wasMultiTouch {
                // Single tap (no drag detected) → left click
                if let touch = touches.first {
                    let point = touch.location(in: view)
                    let (mx, my) = convertToMac(point, in: view)
                    sendTap(button: 0, x: mx, y: my)
                }
            }
            gestureState = .none

        case 1:
            // One finger remaining after lifting one — mouse-up if dragging
            if mouseIsDown {
                sendMouseUp(button: 0)
                mouseIsDown = false
            }
            gestureState = .none

        default:
            break
        }
    }

    /// Call when touches are cancelled.
    func touchesCancelled(_ touches: Set<UITouch>, in view: UIView) {
        for touch in touches {
            let key = ObjectIdentifier(touch)
            activeTouchInfo.removeValue(forKey: key)
        }
        if activeTouchInfo.isEmpty {
            if mouseIsDown {
                sendMouseUp(button: 0)
                mouseIsDown = false
            }
            gestureState = .none
        }
    }

    // MARK: Two-finger interactions

    private func handleRightClick(in view: UIView) {
        // Send right-click at the first touch's location
        guard let first = activeTouchInfo.first else { return }
        let (mx, my) = convertToMac(first.value.currentPoint, in: view)
        sendTap(button: 1, x: mx, y: my)
        gestureState = .none
    }

    private func handleTwoFingerScroll(in view: UIView) {
        let touches = Array(activeTouchInfo.values)
        guard touches.count == 2 else { return }

        let prevAvg = CGPoint(
            x: (touches[0].previousPoint.x + touches[1].previousPoint.x) / 2.0,
            y: (touches[0].previousPoint.y + touches[1].previousPoint.y) / 2.0
        )
        let currAvg = CGPoint(
            x: (touches[0].currentPoint.x + touches[1].currentPoint.x) / 2.0,
            y: (touches[0].currentPoint.y + touches[1].currentPoint.y) / 2.0
        )

        let deltaX = currAvg.x - prevAvg.x
        let deltaY = currAvg.y - prevAvg.y

        // Only send scroll if movement exceeds threshold
        let distance = hypot(deltaX, deltaY)
        if distance > 2.0 || gestureState == .scrolling {
            gestureState = .scrolling
            // Scale delta for natural-feeling scroll speed
            let scale: Double = 3.0
            webRTCService?.sendScroll(deltaX: Double(deltaX) * scale, deltaY: Double(deltaY) * scale)
        }
    }

    // MARK: Coordinate conversion

    /// Convert a point from the iOS view coordinate space to remote macOS coordinates.
    /// - Returns: (macX, macY) in remote display pixels.
    func convertToMac(_ point: CGPoint, in view: UIView) -> (Double, Double) {
        let viewW = max(viewSize.width, 1.0)
        let viewH = max(viewSize.height, 1.0)
        let scaleX = remoteScreenSize.width / viewW
        let scaleY = remoteScreenSize.height / viewH
        let macX = point.x * scaleX
        let macY = point.y * scaleY
        return (Double(macX.clamped(to: 0...remoteScreenSize.width)),
                Double(macY.clamped(to: 0...remoteScreenSize.height)))
    }

    /// Convert a point and return as an integer tuple (for pixel-precise events).
    private func convertToMacInt(_ point: CGPoint, in view: UIView) -> (Int, Int) {
        let (x, y) = convertToMac(point, in: view)
        return (Int(x.rounded()), Int(y.rounded()))
    }

    // MARK: Sending

    private func sendMouseMove(from point: CGPoint, in view: UIView) {
        let (mx, my) = convertToMac(point, in: view)
        // Debounce: skip if position hasn't changed meaningfully
        if let last = lastReportedPosition {
            let dx = abs(mx - last.x)
            let dy = abs(my - last.y)
            if dx < 1.0 && dy < 1.0 { return }
        }
        lastReportedPosition = CGPoint(x: mx, y: my)
        webRTCService?.sendMouseMove(x: mx, y: my)
    }

    private func sendMouseDown(button: Int, x: Double, y: Double) {
        webRTCService?.sendMouseClick(button: button, x: x, y: y, down: true)
    }

    private func sendMouseUp(button: Int) {
        // Use last-known position
        let pos = lastReportedPosition ?? .zero
        webRTCService?.sendMouseClick(button: button, x: pos.x, y: pos.y, down: false)
    }

    /// Send a complete tap (down + up) at the given position.
    private func sendTap(button: Int, x: Double, y: Double) {
        webRTCService?.sendMouseClick(button: button, x: x, y: y, down: true)
        webRTCService?.sendMouseClick(button: button, x: x, y: y, down: false)
    }

    // MARK: Keyboard

    /// Send a key-press event. Call from the render view's `pressesBegan` override.
    /// - Parameters:
    ///   - keyCode: The macOS virtual key code.
    ///   - chars:   The character(s) the key produces, if any.
    func keyPressed(keyCode: UInt16, chars: String? = nil) {
        webRTCService?.sendKey(keyCode: keyCode, chars: chars, down: true)
        // Auto-release after a small delay for tap-like behavior; the host
        // also receives a key-release for a complete press.
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.05) { [weak self] in
            self?.webRTCService?.sendKey(keyCode: keyCode, chars: nil, down: false)
        }
    }

    /// Send a key-down (press) event — call on `pressesBegan`.
    func keyDown(keyCode: UInt16, chars: String? = nil) {
        webRTCService?.sendKey(keyCode: keyCode, chars: chars, down: true)
    }

    /// Send a key-up (release) event — call on `pressesEnded`.
    func keyUp(keyCode: UInt16) {
        webRTCService?.sendKey(keyCode: keyCode, chars: nil, down: false)
    }

    /// Reset tracking state (call when view disappears).
    func reset() {
        activeTouchInfo.removeAll()
        mouseIsDown = false
        lastReportedPosition = nil
        gestureState = .none
    }
}

// MARK: - Supporting Types

private struct TouchInfo {
    var startPoint: CGPoint
    var previousPoint: CGPoint
    var currentPoint: CGPoint
    var startTime: TimeInterval
}

// MARK: - Clamping helper

private extension CGFloat {
    func clamped(to range: ClosedRange<CGFloat>) -> CGFloat {
        return min(max(self, range.lowerBound), range.upperBound)
    }
}
