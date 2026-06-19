//go:build darwin && cgo

package screen

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework ApplicationServices
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <ApplicationServices/ApplicationServices.h>
#include <dlfcn.h>
#include <stdlib.h>

// CGDisplayCreateImage and CGWindowListCreateImage are obsoleted in macOS 15.0
// but present in the shared library at runtime. Use dlsym to bypass the
// compile-time 'unavailable' attribute.

typedef CGImageRef (*cgDisplayCreateImageFunc)(CGDirectDisplayID);
typedef CGImageRef (*cgWindowListCreateImageFunc)(CGRect, CGWindowListOption, CGWindowID, CGWindowImageOption);

static CGImageRef callCGDisplayCreateImage(CGDirectDisplayID displayID) {
    static cgDisplayCreateImageFunc func = NULL;
    if (func == NULL) {
        func = (cgDisplayCreateImageFunc)dlsym(RTLD_DEFAULT, "CGDisplayCreateImage");
    }
    if (func == NULL) return NULL;
    return func(displayID);
}

static CGImageRef callCGWindowListCreateImage(CGRect bounds, CGWindowListOption option, CGWindowID windowID, CGWindowImageOption imageOption) {
    static cgWindowListCreateImageFunc func = NULL;
    if (func == NULL) {
        func = (cgWindowListCreateImageFunc)dlsym(RTLD_DEFAULT, "CGWindowListCreateImage");
    }
    if (func == NULL) return NULL;
    return func(bounds, option, windowID, imageOption);
}

// CGO in Go 1.24 can't compare C opaque types to Go nil, so we provide C helpers.

static int cgImageIsNull(CGImageRef img) { return img == NULL ? 1 : 0; }
static int colorSpaceIsNull(CGColorSpaceRef cs) { return cs == NULL ? 1 : 0; }
static int contextIsNull(CGContextRef ctx) { return ctx == NULL ? 1 : 0; }
static int displayModeIsNull(CGDisplayModeRef mode) { return mode == NULL ? 1 : 0; }

// CGRectIsNull/IsEmpty return _Bool in C; Go can't compare _Bool with untyped int.
static int rectIsNull(CGRect r) { return CGRectIsNull(r) ? 1 : 0; }
static int rectIsEmpty(CGRect r) { return CGRectIsEmpty(r) ? 1 : 0; }
*/
import "C"

import (
	"fmt"
	"image"
	"unsafe"

	"github.com/rs/zerolog/log"
)

// captureDisplay captures a single frame from the given display ID.
func captureDisplay(displayID int) (*image.RGBA, error) {
	mainDisplayID := C.CGMainDisplayID()
	targetDisplay := C.uint32_t(displayID)
	if displayID == 0 {
		targetDisplay = mainDisplayID
	}

	// Validate display
	bounds := C.CGDisplayBounds(targetDisplay)
	if C.rectIsNull(bounds) != 0 || C.rectIsEmpty(bounds) != 0 {
		return nil, fmt.Errorf("display %d is not active or invalid", displayID)
	}

	// Primary: CGDisplayCreateImage via dlsym
	imageRef := C.callCGDisplayCreateImage(targetDisplay)
	if C.cgImageIsNull(imageRef) != 0 && targetDisplay != mainDisplayID {
		log.Warn().Int("display_id", displayID).Msg("Failed to capture requested display, trying main display")
		imageRef = C.callCGDisplayCreateImage(mainDisplayID)
	}
	if C.cgImageIsNull(imageRef) != 0 {
		log.Debug().Msg("CGDisplayCreateImage failed, trying CGWindowListCreateImage")
		imageRef = captureDisplayFallback(targetDisplay)
	}
	if C.cgImageIsNull(imageRef) != 0 {
		return nil, fmt.Errorf("screen capture failed — check Screen Recording permission in System Settings > Privacy & Security > Screen Recording")
	}
	defer C.CGImageRelease(imageRef)

	return cgImageToRGBA(imageRef)
}

// captureDisplayFallback uses CGWindowListCreateImage via dlsym.
func captureDisplayFallback(displayID C.uint32_t) C.CGImageRef {
	bounds := C.CGDisplayBounds(displayID)
	if C.rectIsNull(bounds) != 0 || C.rectIsEmpty(bounds) != 0 {
		return C.callCGDisplayCreateImage(C.CGMainDisplayID())
	}

	return C.callCGWindowListCreateImage(
		bounds,
		C.kCGWindowListOptionOnScreenOnly,
		C.kCGNullWindowID,
		C.kCGWindowImageDefault,
	)
}

// cgImageToRGBA converts a CGImageRef to *image.RGBA via bitmap context.
func cgImageToRGBA(img C.CGImageRef) (*image.RGBA, error) {
	width := int(C.CGImageGetWidth(img))
	height := int(C.CGImageGetHeight(img))
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("captured image has zero dimensions (%dx%d)", width, height)
	}

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))

	colorSpace := C.CGColorSpaceCreateDeviceRGB()
	if C.colorSpaceIsNull(colorSpace) != 0 {
		return nil, fmt.Errorf("CGColorSpaceCreateDeviceRGB failed")
	}
	defer C.CGColorSpaceRelease(colorSpace)

	bitmapInfo := C.CGBitmapInfo(C.kCGImageAlphaPremultipliedLast) | C.kCGBitmapByteOrder32Big
	ctx := C.CGBitmapContextCreate(nil, C.size_t(width), C.size_t(height), 8, C.size_t(width*4), colorSpace, bitmapInfo)
	if C.contextIsNull(ctx) != 0 {
		return nil, fmt.Errorf("CGBitmapContextCreate failed")
	}
	defer C.CGContextRelease(ctx)

	// Flip Y axis (CoreGraphics uses bottom-left origin)
	C.CGContextTranslateCTM(ctx, 0, C.CGFloat(height))
	C.CGContextScaleCTM(ctx, 1, -1)

	C.CGContextDrawImage(ctx, C.CGRectMake(0, 0, C.CGFloat(width), C.CGFloat(height)), img)

	ctxData := C.CGBitmapContextGetData(ctx)
	if ctxData == nil {
		return nil, fmt.Errorf("CGBitmapContextGetData returned NULL")
	}

	C.memcpy(unsafe.Pointer(&rgba.Pix[0]), ctxData, C.size_t(width*height*4))
	return rgba, nil
}

// ListDisplays returns active display IDs.
func ListDisplays() ([]int, error) {
	maxDisplays := C.uint32_t(16)
	displays := make([]C.uint32_t, maxDisplays)
	var displayCount C.uint32_t

	err := C.CGGetOnlineDisplayList(maxDisplays, &displays[0], &displayCount)
	if err != C.kCGErrorSuccess {
		return nil, fmt.Errorf("CGGetOnlineDisplayList failed with error %d", int(err))
	}

	ids := make([]int, displayCount)
	for i := C.uint32_t(0); i < displayCount; i++ {
		ids[i] = int(displays[i])
	}
	return ids, nil
}

// DisplayWidth returns width in points.
func DisplayWidth(displayID int) int {
	return int(C.CGDisplayPixelsWide(C.uint32_t(displayID)))
}

// DisplayHeight returns height in points.
func DisplayHeight(displayID int) int {
	return int(C.CGDisplayPixelsHigh(C.uint32_t(displayID)))
}

// MainDisplayID returns the main display ID.
func MainDisplayID() int {
	return int(C.CGMainDisplayID())
}

// GetScaleFactor returns the backing scale factor (e.g., 1.0, 2.0).
func GetScaleFactor(displayID int) float64 {
	mode := C.CGDisplayCopyDisplayMode(C.uint32_t(displayID))
	if C.displayModeIsNull(mode) != 0 {
		return 1.0
	}
	defer C.CGDisplayModeRelease(mode)

	pixelWidth := int(C.CGDisplayModeGetPixelWidth(mode))
	pixelHeight := int(C.CGDisplayModeGetPixelHeight(mode))
	pointWidth := DisplayWidth(displayID)
	pointHeight := DisplayHeight(displayID)

	if pointWidth == 0 || pointHeight == 0 {
		return 1.0
	}

	scaleX := float64(pixelWidth) / float64(pointWidth)
	scaleY := float64(pixelHeight) / float64(pointHeight)
	if scaleX > scaleY {
		return scaleX
	}
	return scaleY
}
