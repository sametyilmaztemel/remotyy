//go:build !darwin || !cgo

package screen

// AccessibilityError is returned when accessibility permissions are missing.
type AccessibilityError struct {
	Message string
}

func (e *AccessibilityError) Error() string { return e.Message }

// IsAccessibilityEnabled returns false on non-macOS platforms.
func IsAccessibilityEnabled() bool { return false }

// RequestAccessibilityPermission returns an error on non-macOS.
func RequestAccessibilityPermission() error {
	return &AccessibilityError{
		Message: "accessibility input requires macOS with CGO",
	}
}

// EnsureAccessibility returns an error on non-macOS.
func EnsureAccessibility() error {
	return &AccessibilityError{
		Message: "accessibility input requires macOS with CGO",
	}
}

// PointerButton constants.
type PointerButton int

const (
	ButtonLeft   PointerButton = 0
	ButtonRight  PointerButton = 1
	ButtonMiddle PointerButton = 2
)

// MouseMove is a stub for non-macOS builds.
func MouseMove(x, y float64) error {
	return &AccessibilityError{
		Message: "mouse input requires macOS with CGO; current platform is not darwin",
	}
}

// MouseClick is a stub for non-macOS builds.
func MouseClick(button int, x, y float64) error {
	return &AccessibilityError{
		Message: "mouse click requires macOS with CGO; current platform is not darwin",
	}
}

// MouseButtonDown is a stub for non-macOS builds.
func MouseButtonDown(button int, x, y float64) error {
	return &AccessibilityError{
		Message: "mouse input requires macOS with CGO",
	}
}

// MouseButtonUp is a stub for non-macOS builds.
func MouseButtonUp(button int, x, y float64) error {
	return &AccessibilityError{
		Message: "mouse input requires macOS with CGO",
	}
}

// MouseScroll is a stub for non-macOS builds.
func MouseScroll(deltaX, deltaY float64) error {
	return &AccessibilityError{
		Message: "scroll input requires macOS with CGO",
	}
}

// KeyPress is a stub for non-macOS builds.
func KeyPress(keyCode uint16) error {
	return &AccessibilityError{
		Message: "keyboard input requires macOS with CGO",
	}
}

// KeyRelease is a stub for non-macOS builds.
func KeyRelease(keyCode uint16) error {
	return &AccessibilityError{
		Message: "keyboard input requires macOS with CGO",
	}
}

// KeyTap is a stub for non-macOS builds.
func KeyTap(keyCode uint16) error {
	return &AccessibilityError{
		Message: "keyboard input requires macOS with CGO",
	}
}

// KeyPressWithModifiers is a stub for non-macOS builds.
func KeyPressWithModifiers(keyCode uint16, flags uint64) error {
	return &AccessibilityError{
		Message: "keyboard input requires macOS with CGO",
	}
}

// GetMouseLocation is a stub for non-macOS builds.
func GetMouseLocation() (x, y float64, err error) {
	return 0, 0, &AccessibilityError{
		Message: "mouse location requires macOS with CGO",
	}
}

// StringToKeyCode is a stub for non-macOS builds.
func StringToKeyCode(ch string) uint16 {
	return 0
}
