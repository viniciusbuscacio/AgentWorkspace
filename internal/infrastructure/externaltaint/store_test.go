package externaltaint

import (
	"context"
	"testing"

	"aw/internal/domain"
)

func TestStoreRecordsSuspiciousPerScope(t *testing.T) {
	s := NewStore()

	// Low-risk, non-suspicious content does not taint.
	s.Record("turn-a", domain.ExternalContentSafety{RiskLevel: domain.ExternalRiskLow})
	if s.Safety("turn-a").Suspicious {
		t.Fatal("low-risk content must not taint")
	}

	// Suspicious/high-risk content taints its scope only.
	s.Record("turn-a", domain.ExternalContentSafety{Suspicious: true, RiskLevel: domain.ExternalRiskHigh})
	if !s.Safety("turn-a").Suspicious || s.Safety("turn-a").RiskLevel != domain.ExternalRiskHigh {
		t.Fatalf("suspicious content should taint scope: %+v", s.Safety("turn-a"))
	}
	if s.Safety("turn-b").Suspicious {
		t.Fatal("taint must not bleed to another scope")
	}
}

func TestStoreRecordExternalContentUsesCtxScope(t *testing.T) {
	s := NewStore()
	ctx := domain.WithExternalTaintScope(context.Background(), "turn-c")
	s.RecordExternalContent(ctx, domain.ExternalContentResult{Suspicious: true, RiskLevel: domain.ExternalRiskHigh})
	if !s.Safety("turn-c").Suspicious {
		t.Fatal("RecordExternalContent should record under the ctx scope")
	}
}
