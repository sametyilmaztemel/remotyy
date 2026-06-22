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

// DisplayWidth returns display width (stub on non-macOS).
func DisplayWidth(displayID int) int { return 0 }

// DisplayHeight returns display height (stub on non-macOS).
func DisplayHeight(displayID int) int { return 0 }

// MainDisplayID returns main display ID (stub on non-macOS).
func MainDisplayID() int { return 0 }

// GetScaleFactor returns backing scale factor (stub on non-macOS).
func GetScaleFactor(displayID int) float64 { return 1.0 }
