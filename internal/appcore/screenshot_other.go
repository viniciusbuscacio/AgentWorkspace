//go:build !darwin

package appcore

import "fmt"

// nativeWindowPNG is only implemented on macOS (WKWebView snapshot). On other
// platforms the screenshot path falls back to the frontend DOM capture.
func nativeWindowPNG() ([]byte, error) {
	return nil, fmt.Errorf("native webview snapshot not supported on this platform")
}

// nativeScreenCapturePNG is only implemented on macOS.
func nativeScreenCapturePNG() ([]byte, error) {
	return nil, fmt.Errorf("native window capture not supported on this platform")
}
