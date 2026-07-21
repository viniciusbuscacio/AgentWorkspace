package appcore

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder for image.Decode of native captures
	"os"

	xdraw "golang.org/x/image/draw"
)

// screenshotMaxEdge bounds the longest side of a captured screenshot, matching
// the frontend's domToCanvas path. The native WKWebView snapshot is retina
// resolution (~2x), so without this an inline screenshot would be several MB.
const (
	screenshotMaxEdge     = 1280
	screenshotJPEGQuality = 70
)

// scaledJPEGDataURI decodes a PNG screenshot, downscales the longest side to
// screenshotMaxEdge, JPEG-encodes it, and returns a data URI plus the output
// dimensions. Used to keep native-capture screenshots small enough to ship to
// the chat UI and persist in sessionStorage.
func scaledJPEGDataURI(pngBytes []byte) (string, int, int, error) {
	src, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return "", 0, 0, fmt.Errorf("decode screenshot: %w", err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return "", 0, 0, fmt.Errorf("empty screenshot")
	}
	longest := w
	if h > longest {
		longest = h
	}
	dst := src
	outW, outH := w, h
	if longest > screenshotMaxEdge {
		scale := float64(screenshotMaxEdge) / float64(longest)
		outW = int(float64(w) * scale)
		outH = int(float64(h) * scale)
		if outW < 1 {
			outW = 1
		}
		if outH < 1 {
			outH = 1
		}
		scaled := image.NewRGBA(image.Rect(0, 0, outW, outH))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, b, xdraw.Over, nil)
		dst = scaled
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: screenshotJPEGQuality}); err != nil {
		return "", 0, 0, fmt.Errorf("encode screenshot: %w", err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), outW, outH, nil
}

// safeNativeScreenshot attempts the native WKWebView capture without ever
// letting it crash the app: a screenshot must fail soft, never hard. Any Go
// panic (decode/scale) is recovered into a clean fallback, and the whole path
// can be disabled at runtime with aw_NATIVE_SCREENSHOT=0. It returns ok=false
// whenever the caller should fall back to the frontend DOM capture.
//
// Note: a hard SIGSEGV inside the Objective-C snapshot cannot be recovered in
// Go (the webview lives in this process and cannot be isolated). The native
// side is therefore written to fail soft (ARC, @autoreleasepool, @try/@catch,
// nil guards, bounded wait); this wrapper is the Go-side half of that contract.
func (a *App) safeNativeScreenshot() (result map[string]any, ok bool) {
	if os.Getenv("aw_NATIVE_SCREENSHOT") == "0" {
		return nil, false
	}
	defer func() {
		if r := recover(); r != nil {
			result = nil
			ok = false
		}
	}()
	// Prefer a real on-screen capture: it is the only path that preserves the
	// wallpaper "glass" (CSS backdrop-filter), but it needs Screen Recording
	// permission. When that is denied/unavailable it returns an error and we
	// fall back to the in-process WKWebView snapshot (no glass, no permission).
	capturers := []func() ([]byte, error){nativeScreenCapturePNG, nativeWindowPNG}
	if os.Getenv("aw_SCREENSHOT_NO_GLASS") == "1" {
		capturers = []func() ([]byte, error){nativeWindowPNG}
	}
	for _, capture := range capturers {
		png, err := capture()
		if err != nil || len(png) == 0 {
			continue
		}
		dataURI, w, h, err := scaledJPEGDataURI(png)
		if err != nil || dataURI == "" {
			continue
		}
		return map[string]any{"dataUri": dataURI, "width": w, "height": h}, true
	}
	return nil, false
}
