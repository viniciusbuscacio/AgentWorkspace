package application

import (
	"context"
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeAttachmentReader struct {
	res     domain.ExternalContentResult
	handled bool
	err     error
}

func (f fakeAttachmentReader) ReadAttachmentText(_ context.Context, _ domain.Attachment, _ domain.ModelConfig) (domain.ExternalContentResult, bool, error) {
	return f.res, f.handled, f.err
}

func TestAttachmentContextInjectsUntrustedExtractedText(t *testing.T) {
	reader := fakeAttachmentReader{
		res: domain.ExternalContentResult{
			BodyClean:  "the quarterly numbers are 42",
			RiskLevel:  domain.ExternalRiskHigh,
			Suspicious: true,
			Untrusted:  true,
		},
		handled: true,
	}
	out := attachmentContext(context.Background(), reader, nil, []domain.Attachment{{Name: "report.pdf"}}, domain.ModelConfig{})

	for _, want := range []string{"## External attachments", "UNTRUSTED external data", "report.pdf", "risk=high", "the quarterly numbers are 42"} {
		if !strings.Contains(out, want) {
			t.Fatalf("attachment context missing %q:\n%s", want, out)
		}
	}
}

func TestAttachmentContextNilReaderFallsBackToMetadata(t *testing.T) {
	atts := []domain.Attachment{{Name: "shot.png", Type: "image/png", DataURI: "data:image/png;base64,QUJD"}}
	out := attachmentContext(context.Background(), nil, nil, atts, domain.ModelConfig{})
	// Metadata-only behavior: names/mime, no extracted body, marked untrusted.
	if !strings.Contains(out, "External attachments") || !strings.Contains(out, domain.ExternalTrustUntrusted) {
		t.Fatalf("nil reader should fall back to metadata prompt:\n%s", out)
	}
	if out != domain.ExternalAttachmentPrompt(atts) {
		t.Fatalf("nil reader output should equal the metadata prompt")
	}
}

func TestAttachmentContextUnsupportedFallsBackToMetadata(t *testing.T) {
	reader := fakeAttachmentReader{handled: false}
	atts := []domain.Attachment{{Name: "a.bin", Type: "application/octet-stream", DataURI: "data:application/octet-stream;base64,QUJD"}}
	out := attachmentContext(context.Background(), reader, nil, atts, domain.ModelConfig{})
	if !strings.Contains(out, "a.bin") || !strings.Contains(out, domain.ExternalTrustUntrusted) {
		t.Fatalf("unsupported attachment should still get metadata:\n%s", out)
	}
}

type capturingRecorder struct {
	results []domain.ExternalContentResult
}

func (c *capturingRecorder) RecordExternalContent(_ context.Context, result domain.ExternalContentResult) {
	c.results = append(c.results, result)
}

func TestAttachmentContextRecordsHandledResultIntoTaint(t *testing.T) {
	reader := fakeAttachmentReader{
		res:     domain.ExternalContentResult{BodyClean: "x", RiskLevel: domain.ExternalRiskHigh, Suspicious: true, Untrusted: true},
		handled: true,
	}
	rec := &capturingRecorder{}
	out := attachmentContext(context.Background(), reader, rec, []domain.Attachment{{Name: "report.pdf"}}, domain.ModelConfig{})

	if len(rec.results) != 1 || !rec.results[0].Suspicious {
		t.Fatalf("handled suspicious attachment should be recorded once, got %+v", rec.results)
	}
	// The untrusted prompt block must still be produced (Stage A preserved).
	if !strings.Contains(out, "UNTRUSTED external data") {
		t.Fatalf("recording must not drop the untrusted prompt block:\n%s", out)
	}
}

func TestAttachmentContextDoesNotRecordUnsupported(t *testing.T) {
	rec := &capturingRecorder{}
	atts := []domain.Attachment{{Name: "a.bin", Type: "application/octet-stream", DataURI: "data:application/octet-stream;base64,QUJD"}}
	attachmentContext(context.Background(), fakeAttachmentReader{handled: false}, rec, atts, domain.ModelConfig{})
	if len(rec.results) != 0 {
		t.Fatalf("unsupported attachment must not be recorded, got %+v", rec.results)
	}
}

func TestAttachmentLabelSanitizesControlChars(t *testing.T) {
	got := attachmentLabel(domain.Attachment{Name: "evil\nname\t.png"})
	if strings.ContainsAny(got, "\n\t") {
		t.Fatalf("label must not contain control chars: %q", got)
	}
}
