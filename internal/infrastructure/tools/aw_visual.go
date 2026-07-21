package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aw/internal/domain"
	"aw/internal/infrastructure/externalsafe"
)

// The *.read_safe actions are the enforced "read" path for non-text inputs:
// they convert an image/PDF to text and route that text through the same
// externalsafe + taint pipeline as every other external source, so the main
// agent receives ONLY untrusted-labeled text — never the raw pixels/bytes.
//
//   - document.read_safe: deterministic, pure-Go PDF text extraction (no model).
//   - visual.read_safe: isolated, capability-less vision OCR (needs VisualExtractFn).

func documentReadSafeAction(ctx context.Context, args map[string]any, w *workspace) (string, error) {
	path, err := awRequiredStringArg(args, "path")
	if err != nil {
		return "", err
	}
	data, err := w.readSafeBytes(ctx, path)
	if err != nil {
		return "", err
	}

	var text string
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		text, err = extractPDFText(data)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("no extractable text layer in %q (likely a scanned/image PDF); export its pages as images and use visual.read_safe", path)
		}
	default:
		// Other document types are treated as plain text and still quarantined.
		text = string(data)
	}

	return awJSON(w.processExternalContent(ctx, text, externalProcessOptions{
		SourceType: domain.ExternalSourceFile,
		Origin:     path,
		Mode:       domain.ExternalContentModeDistill,
		MaxChars:   externalsafe.DefaultMaxChars,
	}))
}

func visualReadSafeAction(ctx context.Context, args map[string]any, w *workspace) (string, error) {
	if w.visualExtractFn == nil {
		return "", fmt.Errorf("visual.read_safe is not available (no vision model configured)")
	}
	path, err := awRequiredStringArg(args, "path")
	if err != nil {
		return "", err
	}
	data, err := w.readSafeBytes(ctx, path)
	if err != nil {
		return "", err
	}
	mime := imageMimeFromPath(path)
	// Isolated, capability-less transcription. The returned text is untrusted.
	text, err := w.visualExtractFn(ctx, data, mime)
	if err != nil {
		return "", fmt.Errorf("visual transcription failed: %w", err)
	}
	return awJSON(w.processExternalContent(ctx, text, externalProcessOptions{
		SourceType: domain.ExternalSourceFile,
		Origin:     path,
		Mode:       domain.ExternalContentModeDistill,
		MaxChars:   externalsafe.DefaultMaxChars,
	}))
}

// readSafeBytes resolves a path through the sandbox fence and reads it with a
// size cap, mirroring readFile's safety checks.
func (w *workspace) readSafeBytes(ctx context.Context, path string) ([]byte, error) {
	full, err := w.resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, appendFilePermHint(err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%q is a directory, not a file", path)
	}
	if info.Size() > maxReadBytes {
		return nil, fmt.Errorf("file is too large to read (%d bytes, limit %d)", info.Size(), maxReadBytes)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, appendFilePermHint(err)
	}
	return data, nil
}

func imageMimeFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/png"
	}
}
