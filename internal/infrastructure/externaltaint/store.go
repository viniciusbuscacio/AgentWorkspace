// Package externaltaint holds the per-turn external-content taint, shared by the
// tools layer (which records content fetched by tools and reads taint to gate
// sensitive actions) and the application's attachment path (which records
// suspicious attachment content for the same turn). Keying is by an opaque turn
// scope string (see domain.WithExternalTaintScope), so taint is per-turn and
// never bleeds across turns or chats.
package externaltaint

import (
	"context"
	"sync"

	"aw/internal/domain"
)

// Store accumulates the worst external-content risk seen per turn scope.
type Store struct {
	mu     sync.Mutex
	taints map[string]domain.ExternalTaint
}

// NewStore returns an empty taint store.
func NewStore() *Store {
	return &Store{taints: map[string]domain.ExternalTaint{}}
}

// Record folds one external-content observation into the scope's taint. Only
// suspicious/high-risk observations contaminate (see ExternalTaint.WithObservation).
func (s *Store) Record(scope string, safety domain.ExternalContentSafety) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taints == nil {
		s.taints = map[string]domain.ExternalTaint{}
	}
	s.taints[scope] = s.taints[scope].WithObservation(safety)
}

// Safety returns the accumulated taint for a scope as a safety descriptor
// (zero value when the scope is clean).
func (s *Store) Safety(scope string) domain.ExternalContentSafety {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.taints[scope].Safety()
}

// RecordExternalContent satisfies ports.ExternalTaintRecorder: it records the
// content's safety under the turn scope carried on ctx.
func (s *Store) RecordExternalContent(ctx context.Context, result domain.ExternalContentResult) {
	s.Record(domain.ExternalTaintScope(ctx), result.Safety())
}
