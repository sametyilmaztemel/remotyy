//go:build !darwin || !cgo

package screen

import (
	"fmt"
	"image"
	"runtime"
)

// captureDisplay is a stub for non-macOS or non-CGO builds.
// Real implementation requires macOS + CGO.
func captureDisplay(displayID int) (*image.RGBA, error) {
	return nil, fmt.Errorf("screen capture requires macOS with CGO; current platform: %s/%s",
		runtime.GOOS, runtime.GOARCH)
}

// ListDisplays returns display IDs (stub on non-macOS).
func ListDisplays() ([]int, error) {
	return nil, fmt.Errorf("ListDisplays requires macOS with CGO; current platform: %s/%s",
		runtime.GOOS, runtime.GOARCH)
}
