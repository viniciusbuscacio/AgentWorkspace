package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aw/internal/domain"
	"aw/internal/infrastructure/externalsafe"
	"aw/internal/infrastructure/office"
)

// office.* is the enforced path for OOXML documents (.docx/.xlsx/.pptx):
// fs.read would hand the model raw ZIP bytes and fs.write would corrupt the
// archive. Reads route the extracted text through externalsafe + taint like
// every other external source; edits operate on the XML text nodes so
// formatting survives.

// maxOfficeBytes caps how large an Office file the actions will open. Office
// documents routinely exceed the plain-text read cap, so they get their own.
const maxOfficeBytes = 50 << 20

func registerOfficeActions(reg map[string]AwActionHandler) {
	reg["office.read"] = officeReadAction
	reg["office.replace"] = officeReplaceAction
	reg["office.create"] = officeCreateAction
}

func officeReadAction(ctx context.Context, args map[string]any, w *workspace) (string, error) {
	path, err := awRequiredStringArg(args, "path")
	if err != nil {
		return "", err
	}
	data, format, err := w.readOfficeBytes(ctx, path)
	if err != nil {
		return "", err
	}
	var text string
	switch format {
	case "docx":
		text, err = office.DocxText(data)
	case "pptx":
		var slides []string
		slides, err = office.PptxText(data)
		if err == nil {
			var b strings.Builder
			for i, slide := range slides {
				if i > 0 {
					b.WriteString("\n\n")
				}
				fmt.Fprintf(&b, "## Slide %d\n%s", i+1, slide)
			}
			text = b.String()
		}
	case "xlsx":
		text, err = office.XlsxText(data)
	}
	if err != nil {
		return "", err
	}
	processed := w.processExternalContent(ctx, text, externalProcessOptions{
		SourceType: domain.ExternalSourceFile,
		Origin:     path,
		Mode:       domain.ExternalContentModePreserveVerbatim,
		MaxChars:   externalsafe.DefaultMaxChars,
	})
	return awJSON(map[string]any{
		"path":            path,
		"format":          format,
		"content":         text,
		"external_safety": processed.ExternalSafety,
	})
}

func officeReplaceAction(ctx context.Context, args map[string]any, w *workspace) (string, error) {
	path, err := awRequiredStringArg(args, "path")
	if err != nil {
		return "", err
	}
	oldText, err := awRequiredStringArg(args, "oldText")
	if err != nil {
		return "", err
	}
	newText, _ := args["newText"].(string)
	if oldText == newText {
		return "", fmt.Errorf("oldText and newText are identical (no-op)")
	}
	data, format, err := w.readOfficeBytes(ctx, path)
	if err != nil {
		return "", err
	}
	var replaced []byte
	var count int
	switch format {
	case "docx":
		replaced, count, err = office.ReplaceTextInDocx(data, oldText, newText)
	case "pptx":
		replaced, count, err = office.ReplaceTextInPptx(data, oldText, newText)
	case "xlsx":
		replaced, count, err = office.ReplaceTextInXlsx(data, oldText, newText)
	}
	if err != nil {
		return "", err
	}
	if count == 0 {
		if format == "xlsx" {
			return "", fmt.Errorf("oldText not found in any cell of %s (formula cells are never rewritten)", path)
		}
		return "", fmt.Errorf("oldText not found in %s; the document may split it across formatting runs — office.read the file and replace a shorter fragment that keeps one formatting", path)
	}
	if err := w.requireExternalActionGuard(ctx, "office.replace", domain.ExternalActionPersist, map[string]any{
		"path":    path,
		"oldText": oldText,
		"newText": newText,
	}); err != nil {
		return "", err
	}
	approved, err := w.requireConfirmation(ctx, ConfirmRequest{
		Tool:    "office.replace",
		Summary: fmt.Sprintf("Replace %d occurrence(s) in %q", count, path),
		Args:    map[string]any{"path": path, "replacements": count},
	})
	if err != nil {
		return "", err
	}
	if !approved {
		return "", ErrConfirmationDenied
	}
	full, err := w.resolve(ctx, path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", appendFilePermHint(err)
	}
	if err := os.WriteFile(full, replaced, info.Mode().Perm()); err != nil {
		return "", appendFilePermHint(err)
	}
	return awJSON(map[string]any{"path": path, "replacements": count})
}

func officeCreateAction(ctx context.Context, args map[string]any, w *workspace) (string, error) {
	path, err := awRequiredStringArg(args, "path")
	if err != nil {
		return "", err
	}
	format, err := officeFormatFromPath(path)
	if err != nil {
		return "", err
	}
	full, err := w.resolve(ctx, path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(full); err == nil {
		return "", fmt.Errorf("%s already exists; office.create never overwrites — use office.replace to edit it", path)
	}
	var data []byte
	switch format {
	case "docx":
		text, err := awRequiredStringArg(args, "text")
		if err != nil {
			return "", fmt.Errorf("creating a .docx requires \"text\" (one paragraph per line): %w", err)
		}
		if data, err = office.NewDocx(text); err != nil {
			return "", err
		}
	case "xlsx":
		sheets, err := parseXlsxSheets(args["sheets"])
		if err != nil {
			return "", err
		}
		if data, err = office.NewXlsx(sheets); err != nil {
			return "", err
		}
	case "pptx":
		return "", fmt.Errorf("creating a .pptx from scratch is not supported; office.replace can edit an existing presentation")
	}
	if err := w.requireExternalActionGuard(ctx, "office.create", domain.ExternalActionPersist, map[string]any{
		"path": path,
		"args": args,
	}); err != nil {
		return "", err
	}
	approved, err := w.requireConfirmation(ctx, ConfirmRequest{
		Tool:    "office.create",
		Summary: fmt.Sprintf("Create %s (%d bytes)", path, len(data)),
		Args:    map[string]any{"path": path, "bytes": len(data)},
	})
	if err != nil {
		return "", err
	}
	if !approved {
		return "", ErrConfirmationDenied
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", appendFilePermHint(err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", appendFilePermHint(err)
	}
	return awJSON(map[string]any{"path": path, "format": format, "bytesWritten": len(data)})
}

// readOfficeBytes resolves the path through the sandbox fence and reads the
// whole file under the Office-specific size cap.
func (w *workspace) readOfficeBytes(ctx context.Context, path string) ([]byte, string, error) {
	format, err := officeFormatFromPath(path)
	if err != nil {
		return nil, "", err
	}
	full, err := w.resolve(ctx, path)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, "", appendFilePermHint(err)
	}
	if info.IsDir() {
		return nil, "", fmt.Errorf("%q is a directory, not a file", path)
	}
	if info.Size() > maxOfficeBytes {
		return nil, "", fmt.Errorf("file is too large to open (%d bytes, limit %d)", info.Size(), maxOfficeBytes)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, "", appendFilePermHint(err)
	}
	return data, format, nil
}

func officeFormatFromPath(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".docx":
		return "docx", nil
	case ".xlsx":
		return "xlsx", nil
	case ".pptx":
		return "pptx", nil
	default:
		return "", fmt.Errorf("office.* supports .docx, .xlsx and .pptx; use fs.read/fs.write for plain-text files")
	}
}

// parseXlsxSheets coerces the office.create "sheets" argument:
// [{"name": "...", "rows": [["cell", 123], ...]}, ...]
func parseXlsxSheets(value any) ([]office.XlsxSheet, error) {
	usage := `creating a .xlsx requires "sheets": [{"name": "...", "rows": [["a", 1], ...]}]`
	list, ok := value.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("%s", usage)
	}
	sheets := make([]office.XlsxSheet, 0, len(list))
	for _, entry := range list {
		spec, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s", usage)
		}
		name, _ := spec["name"].(string)
		rawRows, ok := spec["rows"].([]any)
		if !ok {
			return nil, fmt.Errorf("%s", usage)
		}
		rows := make([][]string, 0, len(rawRows))
		for _, rawRow := range rawRows {
			cells, ok := rawRow.([]any)
			if !ok {
				return nil, fmt.Errorf("%s", usage)
			}
			row := make([]string, 0, len(cells))
			for _, cell := range cells {
				if cell == nil {
					row = append(row, "")
					continue
				}
				row = append(row, fmt.Sprintf("%v", cell))
			}
			rows = append(rows, row)
		}
		sheets = append(sheets, office.XlsxSheet{Name: name, Rows: rows})
	}
	return sheets, nil
}
