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