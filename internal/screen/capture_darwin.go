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
