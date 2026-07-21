package ports

import (
	"context"

	"aw/internal/domain"
)

// ExternalTaintRecorder records the safety of external content so later
// sensitive tool actions in the SAME turn are gated by enforcement. It lets the
// application's attachment path feed the same per-turn taint the tools use,
// without the application depending on the tools/infrastructure layer.
//
// A nil recorder is valid and means "do not record" (Stage A behavior: the
// content is still sanitized and labeled untrusted in the prompt).
type ExternalTaintRecorder interface {
	RecordExternalContent(ctx context.Context, result domain.ExternalContentResult)
}
