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
