//go:build darwin

package appcore

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework CoreGraphics
#include <stdlib.h>

// Returns a malloc'd PNG buffer of the app's WKWebView content (caller frees),
// or NULL. *outLen receives the byte length. Defined in screenshot_darwin.m.
unsigned char* aw_capture_webview_png(int* outLen);
// Returns a malloc'd PNG of the app window as composited on screen (includes
// backdrop-filter glass; needs Screen Recording permission). NULL on failure.
unsigned char* aw_capture_window_png(int* outLen);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// nativeWindowPNG renders the app's own WKWebView via -takeSnapshotWithConfiguration:
// (the same in-process compositor render Electron's capturePage() uses). Unlike
// the frontend's DOM->SVG screenshot, this faithfully includes backdrop-filter
// (the wallpaper "glass"/background-visibility effect) and needs no Screen
// Recording permission. Must be called off the main thread (it blocks on the
// snapshot completion handler which runs on the main queue).
func nativeWindowPNG() ([]byte, error) {
	var n C.int
	ptr := C.aw_capture_webview_png(&n)
	if ptr == nil || n <= 0 {
		return nil, fmt.Errorf("native webview snapshot unavailable")
	}
	defer C.free(unsafe.Pointer(ptr))
	return C.GoBytes(unsafe.Pointer(ptr), n), nil
}

// nativeScreenCapturePNG captures the app window as composited on screen
// (CGWindowListCreateImage). It is the only path that preserves CSS
// backdrop-filter (the wallpaper "glass" blur), at the cost of requiring Screen
// Recording permission. Returns an error when capture is blocked/unavailable so
// the caller can fall back to the takeSnapshot path. Must be called off-main.
func nativeScreenCapturePNG() ([]byte, error) {
	var n C.int
	ptr := C.aw_capture_window_png(&n)
	if ptr == nil || n <= 0 {
		return nil, fmt.Errorf("native window capture unavailable")
	}
	defer C.free(unsafe.Pointer(ptr))
	return C.GoBytes(unsafe.Pointer(ptr), n), nil
}
