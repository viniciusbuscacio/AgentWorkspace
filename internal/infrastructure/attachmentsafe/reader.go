// Package attachmentsafe turns a user attachment into sanitized, untrusted
// text. It composes the isolated vision OCR (injected, so this package does
// not depend on the agent runtime), pure-Go PDF and DOCX text extraction,
// best-effort plain-text sniffing for everything else, and the externalsafe
// sanitizer. Strong caps bound cost and abuse: size, pages, chars, timeout,
// and a MIME allow-list for images. A file that is neither an image, PDF,
// DOCX nor sniffable text degrades to metadata-only (never an error).
//
// The application wires the same per-turn taint recorder used by tools, so
// suspicious attachment content can gate later sensitive actions in that turn.
package attachmentsafe

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"aw/internal/domain"
	"aw/internal/infrastructure/externalsafe"
	ledongpdf "github.com/ledongthuc/pdf"
	"rsc.io/pdf"
)

const (
	// MaxAttachmentBytes caps decoded attachment size (defense vs huge inputs).
	MaxAttachmentBytes = 16 << 20 // 16 MiB
	// MaxChars caps the sanitized text handed back into the prompt.
	MaxChars = 8000
	// MaxPDFPages bounds how many PDF pages are read.
	MaxPDFPages = 50
	// TranscribeTimeout bounds the isolated OCR call.
	TranscribeTimeout = 60 * time.Second
)

// imageMIMEAllowlist is the set of image types we send to the isolated OCR call.
var imageMIMEAllowlist = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"image/bmp":  true,
}

// TranscribeFunc runs an isolated, capability-less vision transcription.
type TranscribeFunc func(ctx context.Context, cfg domain.ModelConfig, image []byte, mime string) (string, error)

// DistillFunc runs an isolated, capability-less external-content distiller.
type DistillFunc func(ctx context.Context, cfg domain.ModelConfig, req domain.ExternalDistillRequest) (domain.ExternalDistillResult, error)

// Reader implements ports.SafeAttachmentReader.
type Reader struct {
	transcribe TranscribeFunc
	distill    DistillFunc
}

// New builds a Reader. transcribe may be nil — image attachments are then
// reported as unhandled (PDF still works deterministically).
func New(transcribe TranscribeFunc, distill ...DistillFunc) *Reader {
	var d DistillFunc
	if len(distill) > 0 {
		d = distill[0]
	}
	return &Reader{transcribe: transcribe, distill: d}
}

func (r *Reader) ReadAttachmentText(ctx context.Context, attachment domain.Attachment, cfg domain.ModelConfig) (domain.ExternalContentResult, bool, error) {
	mime, data, err := decodeDataURI(attachment.DataURI)
	if err != nil {
		return domain.ExternalContentResult{}, false, err
	}
	if len(data) == 0 {
		return domain.ExternalContentResult{}, false, nil
	}
	if len(data) > MaxAttachmentBytes {
		return domain.ExternalContentResult{}, false, fmt.Errorf("attachment %q is too large (%d bytes, limit %d)", attachment.Name, len(data), MaxAttachmentBytes)
	}

	var text string
	switch {
	case isPDF(mime, attachment.Name):
		text, err = pdfText(data)
		if err != nil {
			return domain.ExternalContentResult{}, false, err
		}
	case isDOCX(mime, attachment.Name):
		text, err = docxText(data)
		if err != nil {
			return domain.ExternalContentResult{}, false, err
		}
	case imageMIMEAllowlist[strings.ToLower(strings.TrimSpace(mime))]:
		if r.transcribe == nil {
			return domain.ExternalContentResult{}, false, nil
		}
		octx, cancel := context.WithTimeout(ctx, TranscribeTimeout)
		text, err = r.transcribe(octx, cfg, data, mime)
		cancel()
		if err != nil {
			return domain.ExternalContentResult{}, false, err
		}
	default:
		// Best effort for everything else, whatever the extension claims:
		// if the bytes read as text (UTF-8, or UTF-16 with BOM), use them;
		// otherwise let the caller fall back to metadata only.
		decoded, ok := textFromBytes(data)
		if !ok {
			return domain.ExternalContentResult{}, false, nil
		}
		text = decoded
	}

	res := externalsafe.SanitizeContent(domain.ExternalContentInput{
		SourceType: domain.ExternalSourceFile,
		Origin:     attachment.Name,
		Content:    text,
		MaxChars:   MaxChars,
	})
	if r.distill != nil && strings.TrimSpace(res.BodyClean) != "" {
		dctx, cancel := context.WithTimeout(ctx, TranscribeTimeout)
		distilled, derr := r.distill(dctx, cfg, domain.ExternalDistillRequest{
			SourceType: domain.ExternalSourceFile,
			Origin:     attachment.Name,
			Mode:       domain.ExternalContentModeDistill,
			Content:    res.BodyClean,
		})
		cancel()
		if derr == nil {
			res = externalsafe.SanitizeContent(domain.ExternalContentInput{
				SourceType: domain.ExternalSourceFile,
				Origin:     attachment.Name,
				Content:    distilled.CleanText,
				MaxChars:   MaxChars,
			})
			res.Warnings = appendAttachmentDistillWarnings(res.Warnings, distilled)
			if len(distilled.PossibleInstructionsFound) > 0 {
				res.Suspicious = true
				res.RiskLevel = domain.ExternalRiskHigh
			}
		}
	}
	return res, true, nil
}

func appendAttachmentDistillWarnings(warnings []string, distilled domain.ExternalDistillResult) []string {
	for _, warning := range distilled.Warnings {
		if warning = strings.TrimSpace(warning); warning != "" {
			warnings = append(warnings, "distiller_warning: "+warning)
		}
	}
	for _, found := range distilled.PossibleInstructionsFound {
		if found = strings.TrimSpace(found); found != "" {
			warnings = append(warnings, "semantic_instruction_found: "+found)
		}
	}
	if warnings == nil {
		return []string{}
	}
	return warnings
}

func isPDF(mime, name string) bool {
	if strings.Contains(strings.ToLower(mime), "pdf") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), ".pdf")
}

func isDOCX(mime, name string) bool {
	if strings.Contains(strings.ToLower(mime), "wordprocessingml") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), ".docx")
}

// docxText extracts the plain text of a .docx (a ZIP holding
// word/document.xml) with only the stdlib: paragraphs become lines, tabs and
// explicit breaks are preserved. Panics from hostile archives are recovered.
func docxText(data []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			text = ""
			err = fmt.Errorf("could not parse DOCX (malformed or unsupported): %v", r)
		}
	}()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("could not open DOCX: %w", err)
	}
	var document io.ReadCloser
	for _, file := range archive.File {
		if file.Name == "word/document.xml" {
			document, err = file.Open()
			if err != nil {
				return "", fmt.Errorf("could not read DOCX document: %w", err)
			}
			break
		}
	}
	if document == nil {
		return "", fmt.Errorf("not a DOCX: word/document.xml is missing")
	}
	defer func() { _ = document.Close() }()

	// The XML never legitimately exceeds the (already capped) archive size by
	// much; a hard cap defends against decompression bombs.
	decoder := xml.NewDecoder(io.LimitReader(document, MaxAttachmentBytes))
	var b strings.Builder
	inText := false
	for {
		token, terr := decoder.Token()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			return "", fmt.Errorf("could not parse DOCX document: %w", terr)
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				b.WriteString("\t")
			case "br":
				b.WriteString("\n")
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteString("\n")
			}
		case xml.CharData:
			if inText {
				b.Write(t)
			}
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// textFromBytes decides whether raw bytes are readable text and decodes them:
// UTF-8 (with or without BOM) and BOM-marked UTF-16 pass; NUL bytes or a
// meaningful share of control characters mean binary.
func textFromBytes(data []byte) (string, bool) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if decoded, ok := decodeUTF16WithBOM(data); ok {
		data = []byte(decoded)
	}
	if len(data) == 0 || !utf8.Valid(data) {
		return "", false
	}
	control := 0
	for _, r := range string(data) {
		if r == 0 {
			return "", false
		}
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			control++
		}
	}
	if control*100 > len(data) {
		// More than ~1% control characters reads as binary, not text.
		return "", false
	}
	return string(data), true
}

func decodeUTF16WithBOM(data []byte) (string, bool) {
	if len(data) < 2 || len(data)%2 != 0 {
		return "", false
	}
	var littleEndian bool
	switch {
	case data[0] == 0xFF && data[1] == 0xFE:
		littleEndian = true
	case data[0] == 0xFE && data[1] == 0xFF:
		littleEndian = false
	default:
		return "", false
	}
	units := make([]uint16, 0, (len(data)-2)/2)
	for i := 2; i+1 < len(data); i += 2 {
		if littleEndian {
			units = append(units, uint16(data[i])|uint16(data[i+1])<<8)
		} else {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		}
	}
	return string(utf16.Decode(units)), true
}

// decodeDataURI parses a "data:<mime>;base64,<payload>" URI.
func decodeDataURI(dataURI string) (string, []byte, error) {
	s := strings.TrimSpace(dataURI)
	if !strings.HasPrefix(s, "data:") {
		return "", nil, fmt.Errorf("attachment is not a data URI")
	}
	s = strings.TrimPrefix(s, "data:")
	meta, payload, ok := strings.Cut(s, ",")
	if !ok {
		return "", nil, fmt.Errorf("malformed data URI")
	}
	mime := meta
	base64Encoded := false
	if i := strings.IndexByte(meta, ';'); i >= 0 {
		mime = meta[:i]
		base64Encoded = strings.Contains(meta[i:], "base64")
	}
	if base64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
		if err != nil {
			return "", nil, fmt.Errorf("invalid base64 in data URI: %w", err)
		}
		return mime, decoded, nil
	}
	return mime, []byte(payload), nil
}

// pdfText extracts a PDF's text. It tries ledongthuc/pdf first (handles modern
// PDFs with object/xref streams and CID fonts, giving clean text) and falls
// back to rsc.io/pdf when that yields nothing. Both paths recover from library
// panics so a hostile PDF cannot crash the process.
func pdfText(data []byte) (string, error) {
	if text := ledongPDFText(data); strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text), nil
	}
	return rscPDFText(data)
}

// ledongPDFText extracts via ledongthuc/pdf, swallowing errors/panics so the
// caller can fall back. Returns "" when it cannot read the PDF.
func ledongPDFText(data []byte) (out string) {
	defer func() {
		if recover() != nil {
			out = ""
		}
	}()
	reader, err := ledongpdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	pages := reader.NumPage()
	if pages > MaxPDFPages {
		pages = MaxPDFPages
	}
	var b strings.Builder
	for i := 1; i <= pages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, terr := page.GetPlainText(nil)
		if terr != nil {
			continue
		}
		b.WriteString(text)
		if i < pages {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// rscPDFText is the original rsc.io/pdf extractor, kept as a fallback. It
// recovers from the library's panics and caps pages.
func rscPDFText(data []byte) (text string, err error) {
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
	if pages > MaxPDFPages {
		pages = MaxPDFPages
	}
	for i := 1; i <= pages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		var lastY float64
		for j, t := range page.Content().Text {
			if j > 0 {
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
