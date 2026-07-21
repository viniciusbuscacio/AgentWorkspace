package appcore

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestScaledJPEGDataURIDownscalesLargeCapture(t *testing.T) {
	dataURI, w, h, err := scaledJPEGDataURI(makePNG(t, 2560, 1600))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dataURI, "data:image/jpeg;base64,") {
		t.Fatalf("expected a jpeg data URI, got %q", dataURI[:32])
	}
	if w != screenshotMaxEdge {
		t.Fatalf("longest side should be clamped to %d, got %d", screenshotMaxEdge, w)
	}
	if h != 800 {
		t.Fatalf("aspect ratio should be preserved (expected 800), got %d", h)
	}
	// A downscaled JPEG must be far smaller than the source retina PNG.
	if len(dataURI) > 400_000 {
		t.Fatalf("downscaled screenshot unexpectedly large: %d bytes", len(dataURI))
	}
}

func TestScaledJPEGDataURIKeepsSmallCapture(t *testing.T) {
	_, w, h, err := scaledJPEGDataURI(makePNG(t, 800, 600))
	if err != nil {
		t.Fatal(err)
	}
	if w != 800 || h != 600 {
		t.Fatalf("small captures must not be upscaled, got %dx%d", w, h)
	}
}

func TestScaledJPEGDataURIRejectsGarbage(t *testing.T) {
	if _, _, _, err := scaledJPEGDataURI([]byte("not an image")); err == nil {
		t.Fatal("expected an error decoding non-image bytes")
	}
}
