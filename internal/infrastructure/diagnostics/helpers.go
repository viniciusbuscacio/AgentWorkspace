package diagnostics

import (
	"sort"

	"aw/internal/domain"
)

// topProcesses sorts a copy of in by key descending and returns the first limit
// rows (never nil). Shared by every OS backend.
func topProcesses(in []domain.DiagnosticsProcess, limit int, key func(domain.DiagnosticsProcess) float64) []domain.DiagnosticsProcess {
	out := append([]domain.DiagnosticsProcess(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return key(out[i]) > key(out[j]) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []domain.DiagnosticsProcess{}
	}
	return out
}

// roundTo rounds v to the given number of decimal places.
//
//nolint:unused // used by Linux/Windows diagnostics backends; Darwin keeps the shared package compiling.
func roundTo(v float64, places int) float64 {
	p := 1.0
	for i := 0; i < places; i++ {
		p *= 10
	}
	if v < 0 {
		return float64(int64(v*p-0.5)) / p
	}
	return float64(int64(v*p+0.5)) / p
}

// topKeys returns the n highest-count keys, ties broken alphabetically.
//
//nolint:unused // used by Linux/Windows diagnostics backends; Darwin keeps the shared package compiling.
func topKeys(counts map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(counts))
	for k, v := range counts {
		pairs = append(pairs, kv{k, v})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	out := []string{}
	for i := 0; i < len(pairs) && i < n; i++ {
		out = append(out, pairs[i].k)
	}
	return out
}

// keysOf returns the set's keys, sorted.
//
//nolint:unused // used by Linux/Windows diagnostics backends; Darwin keeps the shared package compiling.
func keysOf(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
