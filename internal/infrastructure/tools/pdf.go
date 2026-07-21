package tools

import (
	"bytes"
	"fmt"
	"strings"

	"rsc.io/pdf"
)

// extractPDFText pulls the visible text from a text-layer PDF using the pure-Go
// rsc.io/pdf (The Go Authors, BSD-3 — no CGo, offline, deterministic, and immune
// to prompt injection since it only parses text objects). It recovers from the
// library's panics on malformed input so a hostile PDF cannot crash the agent.
//
// Scanned/image-only PDFs have no text layer and return an empty string: pure Go
// cannot rasterize pages for vision OCR, so the caller must fall back to
// visual.read_safe on page images for those.
func extractPDFText(data []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			text = ""
			err = fmt.Errorf("could not parse PDF (malformed or unsupported): %v", r)
		}
	}()
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("could not open PDF: %w", err)
	}
	var b strings.Builder
	pages := reader.NumPage()
	for i := 1; i <= pages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		content := page.Content()
		var lastY float64
		for j, t := range content.Text {
			if j > 0 {
				// A meaningful change in the Y baseline is a new line; otherwise
				// keep fragments on the same line separated by a space.
				if absFloat(t.Y-lastY) > 3 {
					b.WriteString("\n")
				} else {
					b.WriteString(" ")
				}
			}
			b.WriteString(t.S)
			lastY = t.Y
		}
		if i < pages {
			b.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
