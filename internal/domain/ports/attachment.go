package ports

import (
	"context"

	"aw/internal/domain"
)

// SafeAttachmentReader converts a user attachment into sanitized, untrusted
// text so the agent can use its content without ever receiving the raw bytes.
// The implementation lives in infrastructure (isolated OCR for images, pure-Go
// PDF and DOCX text extraction, best-effort text sniffing for anything else,
// then externalsafe).
//
// The extracted text is sanitized and LABELED as untrusted external content and
// injected into the prompt. When a ports.ExternalTaintRecorder is also wired
// (Stage C), suspicious/high-risk attachment content additionally feeds the
// per-turn taint, so later sensitive tool actions in the same turn are gated by
// enforcement — not only by the model treating the content as data.
type SafeAttachmentReader interface {
	// ReadAttachmentText returns the sanitized text for one attachment. The bool
	// reports whether the content could be extracted (image/PDF/DOCX/text);
	// binary types return (zero, false, nil) so the caller falls back to
	// metadata only.
	ReadAttachmentText(ctx context.Context, attachment domain.Attachment, cfg domain.ModelConfig) (domain.ExternalContentResult, bool, error)
}
