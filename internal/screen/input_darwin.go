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

// StringToKeyCode maps a character to a virtual key code.
func StringToKeyCode(ch string) uint16 {
	if len(ch) == 0 {
		return 0
	}
	c := ch[0]
	switch {
	case c >= 'a' && c <= 'z':
		return uint16(0x00 + int(c-'a'))
	case c >= 'A' && c <= 'Z':
		return uint16(0x00 + int(c-'A'))
	case c >= '0' && c <= '9':
		return uint16(0x1D + int(c-'0'))
	default:
		switch c {
		case ' ':
			return 0x31
		case '\n', '\r':
			return 0x24
		case '\t':
			return 0x30
		case '.':
			return 0x2F
		case ',':
			return 0x2B
		case '-':
			return 0x1B
		case '=':
			return 0x18
		case '/':
			return 0x2C
		case ';':
			return 0x29
		case '\'':
			return 0x27
		case '[':
			return 0x21
		case ']':
			return 0x1E
		case '\\':
			return 0x2A
		case '`':
			return 0x32
		}
		return 0
	}
}

// Virtual key code constants.
const (
	kVK_ANSI_A            = 0x00
	kVK_ANSI_S            = 0x01
	kVK_ANSI_D            = 0x02
	kVK_ANSI_F            = 0x03
	kVK_ANSI_H            = 0x04
	kVK_ANSI_G            = 0x05
	kVK_ANSI_Z            = 0x06
	kVK_ANSI_X            = 0x07
	kVK_ANSI_C            = 0x08
	kVK_ANSI_V            = 0x09
	kVK_ANSI_B            = 0x0B
	kVK_ANSI_Q            = 0x0C
	kVK_ANSI_W            = 0x0D
	kVK_ANSI_E            = 0x0E
	kVK_ANSI_R            = 0x0F
	kVK_ANSI_Y            = 0x10
	kVK_ANSI_T            = 0x11
	kVK_ANSI_1            = 0x12
	kVK_ANSI_2            = 0x13
	kVK_ANSI_3            = 0x14
	kVK_ANSI_4            = 0x15
	kVK_ANSI_6            = 0x16
	kVK_ANSI_5            = 0x17
	kVK_ANSI_Equal        = 0x18
	kVK_ANSI_9            = 0x19
	kVK_ANSI_7            = 0x1A
	kVK_ANSI_Minus        = 0x1B
	kVK_ANSI_8            = 0x1C
	kVK_ANSI_0            = 0x1D
	kVK_ANSI_RightBracket = 0x1E
	kVK_ANSI_O            = 0x1F
	kVK_ANSI_U            = 0x20
	kVK_ANSI_LeftBracket  = 0x21
	kVK_ANSI_I            = 0x22
	kVK_ANSI_P            = 0x23
	kVK_Return            = 0x24
	kVK_ANSI_L            = 0x25
	kVK_ANSI_J            = 0x26
	kVK_ANSI_Quote        = 0x27
	kVK_ANSI_K            = 0x28
	kVK_ANSI_Semicolon    = 0x29
	kVK_ANSI_Backslash    = 0x2A
	kVK_ANSI_Comma        = 0x2B
	kVK_ANSI_Slash        = 0x2C
	kVK_ANSI_N            = 0x2D
	kVK_ANSI_M            = 0x2E
	kVK_ANSI_Period       = 0x2F
	kVK_Tab               = 0x30
	kVK_Space             = 0x31
	kVK_ANSI_Grave        = 0x32
	kVK_Delete            = 0x33
	kVK_Escape            = 0x35
	kVK_Command           = 0x37
	kVK_Shift             = 0x38
	kVK_CapsLock          = 0x39
	kVK_Option            = 0x3A
	kVK_Control           = 0x3B
	kVK_RightShift        = 0x3C
	kVK_RightOption       = 0x3D
	kVK_RightControl      = 0x3E
	kVK_Function          = 0x3F
	kVK_F1                = 0x7A
	kVK_F2                = 0x78
	kVK_F3                = 0x63
	kVK_F4                = 0x76
	kVK_F5                = 0x60
	kVK_F6                = 0x61
	kVK_F7                = 0x62
	kVK_F8                = 0x64
	kVK_F9                = 0x65
	kVK_F10               = 0x6D
	kVK_F11               = 0x67
	kVK_F12               = 0x6F
	kVK_UpArrow           = 0x7E
	kVK_DownArrow         = 0x7D
	kVK_LeftArrow         = 0x7B
	kVK_RightArrow        = 0x7C
	kVK_PageUp            = 0x74
	kVK_PageDown          = 0x79
	kVK_Home              = 0x73
	kVK_End               = 0x77
	kVK_ForwardDelete     = 0x75
	kVK_Help              = 0x72
	kVK_Mute              = 0x4A
	kVK_VolumeUp          = 0x48
	kVK_VolumeDown        = 0x49
)

// suppress deadcode warnings for key code constants
var _ = []uint16{
	kVK_ANSI_A, kVK_ANSI_S, kVK_ANSI_D, kVK_ANSI_F, kVK_ANSI_H, kVK_ANSI_G,
	kVK_ANSI_Z, kVK_ANSI_X, kVK_ANSI_C, kVK_ANSI_V, kVK_ANSI_B, kVK_ANSI_Q,
	kVK_ANSI_W, kVK_ANSI_E, kVK_ANSI_R, kVK_ANSI_Y, kVK_ANSI_T, kVK_ANSI_1,
	kVK_ANSI_2, kVK_ANSI_3, kVK_ANSI_4, kVK_ANSI_6, kVK_ANSI_5, kVK_ANSI_Equal,
	kVK_ANSI_9, kVK_ANSI_7, kVK_ANSI_Minus, kVK_ANSI_8, kVK_ANSI_0,
	kVK_ANSI_RightBracket, kVK_ANSI_O, kVK_ANSI_U, kVK_ANSI_LeftBracket,
	kVK_ANSI_I, kVK_ANSI_P, kVK_Return, kVK_ANSI_L, kVK_ANSI_J, kVK_ANSI_Quote,
	kVK_ANSI_K, kVK_ANSI_Semicolon, kVK_ANSI_Backslash, kVK_ANSI_Comma,
	kVK_ANSI_Slash, kVK_ANSI_N, kVK_ANSI_M, kVK_ANSI_Period, kVK_Tab, kVK_Space,
	kVK_ANSI_Grave, kVK_Delete, kVK_Escape, kVK_Command, kVK_Shift, kVK_CapsLock,
	kVK_Option, kVK_Control, kVK_RightShift, kVK_RightOption, kVK_RightControl,
	kVK_Function, kVK_F1, kVK_F2, kVK_F3, kVK_F4, kVK_F5, kVK_F6, kVK_F7, kVK_F8,
	kVK_F9, kVK_F10, kVK_F11, kVK_F12, kVK_UpArrow, kVK_DownArrow, kVK_LeftArrow,
	kVK_RightArrow, kVK_PageUp, kVK_PageDown, kVK_Home, kVK_End, kVK_ForwardDelete,
	kVK_Help, kVK_Mute, kVK_VolumeUp, kVK_VolumeDown,
}
