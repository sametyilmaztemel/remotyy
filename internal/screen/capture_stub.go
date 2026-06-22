//go:build !darwin || !cgo

package screen

import (
	"fmt"
	"image"
	"runtime"
)

// captureDisplay is a stub for non-macOS or non-CGO builds.