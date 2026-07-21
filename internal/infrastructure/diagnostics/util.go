package diagnostics

import (
	"fmt"
	"strings"
)

// stringsBuilder is a tiny line-oriented builder for the markdown summary.
type stringsBuilder struct{ b strings.Builder }

func (s *stringsBuilder) line(text string) {
	if s.b.Len() > 0 {
		s.b.WriteByte('\n')
	}
	s.b.WriteString(text)
}

func (s *stringsBuilder) String() string { return s.b.String() }

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return strings.TrimSpace(s)
}

// humanBytes renders a byte count as a compact GiB/MiB string.
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit && exp < 4; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}

func formatPercent(p float64) string {
	return fmt.Sprintf("%.0f%%", p)
}

// firstLine returns the first non-empty line of s, trimmed.
//
//nolint:unused // used by Linux/Windows diagnostics backends; Darwin keeps the shared package compiling.
func firstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			return t
		}
	}
	return ""
}
