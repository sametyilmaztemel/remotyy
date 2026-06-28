//go:build darwin && cgo

package screen

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework ApplicationServices
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <ApplicationServices/ApplicationServices.h>
#include <dlfcn.h>

// Accessibility helpers

static int isAccessibilityEnabled(void) {
    return (int)AXIsProcessTrusted();
}

static int checkAccessibilityWithPrompt(void) {
    const void *keys[] = { CFSTR("AXTrustedCheckOptionPrompt") };
    int one = 1;
    const void *values[] = { CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &one) };
    CFDictionaryRef options = CFDictionaryCreate(
        kCFAllocatorDefault, keys, values, 1,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks
    );
    int result = (int)AXIsProcessTrustedWithOptions(options);
    CFRelease((CFTypeRef)values[0]);
    CFRelease(options);
    return result;
}

// CGO can't compare C opaque types to Go nil in Go 1.24+, so we use C helpers.

static int eventIsNull(CGEventRef ev) { return ev == NULL ? 1 : 0; }

// Variadic C wrapper: CGEventCreateScrollWheelEvent is variadic.
static CGEventRef createScrollWheelEvent(int32_t deltaY, int32_t deltaX) {
    return CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitPixel, 2, deltaY, deltaX);
}

// Value-type wrappers to avoid CGO pointer type issues.
static CGEventRef createMouseEvent(CGEventType type, double x, double y, uint32_t button) {
    return CGEventCreateMouseEvent(NULL, type, CGPointMake((CGFloat)x, (CGFloat)y), button);
}

static CGEventRef createKeyboardEvent(CGKeyCode keyCode, bool keyDown) {
    return CGEventCreateKeyboardEvent(NULL, keyCode, keyDown);
}

static CGEventRef createEvent(void) {
    return CGEventCreate(NULL);
}

// CGEventPost returns void on macOS 15+ (was CGError on older macOS).
static void postEvent(CGEventRef event) {
    CGEventPost(kCGHIDEventTap, event);
}
*/
import "C"

import (
	"fmt"
	"time"
)

// AccessibilityError is returned when accessibility permissions are missing.
type AccessibilityError struct {
	Message string
}

func (e *AccessibilityError) Error() string { return e.Message }

// IsAccessibilityEnabled returns true if the process has accessibility permissions.
func IsAccessibilityEnabled() bool {
	return C.isAccessibilityEnabled() != 0
}

// RequestAccessibilityPermission prompts the user to grant accessibility permissions.
func RequestAccessibilityPermission() error {
	result := C.checkAccessibilityWithPrompt()
	if result != 0 {
		return nil
	}
	return &AccessibilityError{
		Message: "accessibility permission denied — enable in System Settings > Privacy & Security > Accessibility",
	}
}

// EnsureAccessibility waits for accessibility permission, polling up to timeout.
func EnsureAccessibility(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsAccessibilityEnabled() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return &AccessibilityError{
		Message: fmt.Sprintf("accessibility permission not granted within %v", timeout),
	}
}

// PointerButton constants.
type PointerButton int

const (
	ButtonLeft   PointerButton = 0
	ButtonRight  PointerButton = 1
	ButtonMiddle PointerButton = 2
)

// MouseMove moves the cursor to coordinates (x, y).
func MouseMove(x, y float64) error {
	if !IsAccessibilityEnabled() {
		return &AccessibilityError{Message: "cannot move mouse without accessibility permission"}
	}

	event := C.createMouseEvent(C.kCGEventMouseMoved, C.double(x), C.double(y), 0)
	if C.eventIsNull(event) != 0 {
		return fmt.Errorf("CGEventCreateMouseEvent returned NULL")
	}
	defer C.CFRelease(C.CFTypeRef(event))

	C.postEvent(event)
	return nil
}

// MouseClick sends button down + up at coordinates (x, y).
// button: 0=left, 1=right, 2=middle.
func MouseClick(button int, x, y float64) error {
	if !IsAccessibilityEnabled() {
		return &AccessibilityError{Message: "cannot click without accessibility permission"}
	}

	// Get current mouse position
	event := C.createEvent()
	if C.eventIsNull(event) != 0 {
		return fmt.Errorf("CGEventCreate returned NULL")
	}
	curPoint := C.CGEventGetLocation(event)
	C.CFRelease(C.CFTypeRef(event))

	// Use provided coordinates if specified
	if x > 0 || y > 0 {
		curPoint = C.CGPointMake(C.CGFloat(x), C.CGFloat(y))
		_ = MouseMove(x, y)
	}

	mouseButton := C.uint32_t(button)
	eventDownType := C.CGEventType(C.kCGEventLeftMouseDown + C.uint32_t(button*2))
	eventUpType := C.CGEventType(C.kCGEventLeftMouseUp + C.uint32_t(button*2))

	// Down
	downEvent := C.createMouseEvent(eventDownType, C.double(float64(curPoint.x)), C.double(float64(curPoint.y)), mouseButton)
	if C.eventIsNull(downEvent) != 0 {
		return fmt.Errorf("CGEventCreateMouseEvent(down) returned NULL")
	}
	C.postEvent(downEvent)
	C.CFRelease(C.CFTypeRef(downEvent))

	time.Sleep(10 * time.Millisecond)

	// Up
	upEvent := C.createMouseEvent(eventUpType, C.double(float64(curPoint.x)), C.double(float64(curPoint.y)), mouseButton)
	if C.eventIsNull(upEvent) != 0 {
		return fmt.Errorf("CGEventCreateMouseEvent(up) returned NULL")
	}
	C.postEvent(upEvent)
	C.CFRelease(C.CFTypeRef(upEvent))

	return nil
}

// MouseButtonDown sends a mouse button press (without release).
func MouseButtonDown(button int, x, y float64) error {
	if !IsAccessibilityEnabled() {
		return &AccessibilityError{Message: "cannot send mouse event without accessibility permission"}
	}

	eventType := C.CGEventType(C.kCGEventLeftMouseDown + C.uint32_t(button*2))
	event := C.createMouseEvent(eventType, C.double(x), C.double(y), C.uint32_t(button))
	if C.eventIsNull(event) != 0 {
		return fmt.Errorf("CGEventCreateMouseEvent(down) returned NULL")
	}
	defer C.CFRelease(C.CFTypeRef(event))

	C.postEvent(event)
	return nil
}

// MouseButtonUp sends a mouse button release.
func MouseButtonUp(button int, x, y float64) error {
	if !IsAccessibilityEnabled() {
		return &AccessibilityError{Message: "cannot send mouse event without accessibility permission"}
	}

	eventType := C.CGEventType(C.kCGEventLeftMouseUp + C.uint32_t(button*2))
	event := C.createMouseEvent(eventType, C.double(x), C.double(y), C.uint32_t(button))
	if C.eventIsNull(event) != 0 {
		return fmt.Errorf("CGEventCreateMouseEvent(up) returned NULL")
	}
	defer C.CFRelease(C.CFTypeRef(event))

	C.postEvent(event)
	return nil
}

// MouseScroll sends a scroll wheel event.
// Positive deltaY = scroll up, negative = scroll down.
func MouseScroll(deltaX, deltaY float64) error {
	if !IsAccessibilityEnabled() {
		return &AccessibilityError{Message: "cannot scroll without accessibility permission"}
	}

	scrollEvent := C.createScrollWheelEvent(C.int32_t(-deltaY), C.int32_t(deltaX))
	if C.eventIsNull(scrollEvent) != 0 {
		return fmt.Errorf("createScrollWheelEvent returned NULL")
	}
	defer C.CFRelease(C.CFTypeRef(scrollEvent))

	C.postEvent(scrollEvent)
	return nil
}

// KeyPress sends a key down event.
func KeyPress(keyCode uint16) error {
	if !IsAccessibilityEnabled() {
		return &AccessibilityError{Message: "cannot send key event without accessibility permission"}
	}

	event := C.createKeyboardEvent(C.CGKeyCode(keyCode), C.bool(true))
	if C.eventIsNull(event) != 0 {
		return fmt.Errorf("CGEventCreateKeyboardEvent(down) returned NULL")
	}
	defer C.CFRelease(C.CFTypeRef(event))

	C.postEvent(event)
	return nil
}

// KeyRelease sends a key up event.
func KeyRelease(keyCode uint16) error {
	if !IsAccessibilityEnabled() {
		return &AccessibilityError{Message: "cannot send key event without accessibility permission"}
	}

	event := C.createKeyboardEvent(C.CGKeyCode(keyCode), C.bool(false))
	if C.eventIsNull(event) != 0 {
		return fmt.Errorf("CGEventCreateKeyboardEvent(up) returned NULL")
	}
	defer C.CFRelease(C.CFTypeRef(event))

	C.postEvent(event)
	return nil
}

// KeyTap sends a complete key press (down + up).
func KeyTap(keyCode uint16) error {
	if err := KeyPress(keyCode); err != nil {
		return err
	}
	time.Sleep(5 * time.Millisecond)
	return KeyRelease(keyCode)
}

// KeyPressWithModifiers sends a key down event with modifier flags.
func KeyPressWithModifiers(keyCode uint16, flags uint64) error {
	if !IsAccessibilityEnabled() {
		return &AccessibilityError{Message: "cannot send key event without accessibility permission"}
	}

	event := C.createKeyboardEvent(C.CGKeyCode(keyCode), C.bool(true))
	if C.eventIsNull(event) != 0 {
		return fmt.Errorf("CGEventCreateKeyboardEvent(down) returned NULL")
	}
	defer C.CFRelease(C.CFTypeRef(event))

	var cgFlags C.CGEventFlags
	if flags&(1<<0) != 0 {
		cgFlags |= C.kCGEventFlagMaskCommand
	}
	if flags&(1<<1) != 0 {
		cgFlags |= C.kCGEventFlagMaskShift
	}
	if flags&(1<<2) != 0 {
		cgFlags |= C.kCGEventFlagMaskAlternate
	}
	if flags&(1<<3) != 0 {
		cgFlags |= C.kCGEventFlagMaskControl
	}
	if flags&(1<<4) != 0 {
		cgFlags |= C.kCGEventFlagMaskSecondaryFn
	}
	if flags&(1<<5) != 0 {
		cgFlags |= C.kCGEventFlagMaskNumericPad
	}

	C.CGEventSetFlags(event, cgFlags)

	C.postEvent(event)
	return nil
}

// GetMouseLocation returns the current mouse cursor position.
func GetMouseLocation() (x, y float64, err error) {
	event := C.createEvent()
	if C.eventIsNull(event) != 0 {
		return 0, 0, fmt.Errorf("CGEventCreate returned NULL")
	}
	defer C.CFRelease(C.CFTypeRef(event))

	point := C.CGEventGetLocation(event)
	return float64(point.x), float64(point.y), nil
}