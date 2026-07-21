package diagnostics

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"aw/internal/domain"
)

// parseMeminfo parses /proc/meminfo values (in kB) keyed by name.
func parseMeminfo(content string) map[string]uint64 {
	out := map[string]uint64{}
	for _, line := range strings.Split(content, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		if v, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
			out[key] = v
		}
	}
	return out
}

// journalSince maps a controlled since token to a journalctl --since argument.
func journalSince(token string) string {
	switch token {
	case "15m":
		return "-15 min ago"
	case "1h":
		return "-1 hour ago"
	case "6h":
		return "-6 hour ago"
	case "24h":
		return "-1 day ago"
	case "7d":
		return "-7 day ago"
	default:
		return "-1 hour ago"
	}
}

// journalPriority returns the most permissive journald priority covering the
// requested severities (0 emerg … 7 debug). warning ⇒ 4, error ⇒ 3, critical ⇒ 2.
func journalPriority(sev []string) string {
	max := 0
	for _, s := range sev {
		switch s {
		case "critical":
			max = maxInt(max, 2)
		case "error":
			max = maxInt(max, 3)
		case "warning":
			max = maxInt(max, 4)
		case "info":
			max = maxInt(max, 6)
		}
	}
	if max == 0 {
		max = 4
	}
	return strconv.Itoa(max)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type journalEntry struct {
	Message  json.RawMessage `json:"MESSAGE"`
	Priority string          `json:"PRIORITY"`
	Unit     string          `json:"_SYSTEMD_UNIT"`
	Comm     string          `json:"_COMM"`
	Realtime string          `json:"__REALTIME_TIMESTAMP"`
}

func parseJournalLine(line string) domain.DiagnosticsLogEvent {
	var e journalEntry
	if json.Unmarshal([]byte(line), &e) != nil {
		return domain.DiagnosticsLogEvent{}
	}
	provider := e.Unit
	if provider == "" {
		provider = e.Comm
	}
	return domain.DiagnosticsLogEvent{
		Timestamp: journalTimestamp(e.Realtime),
		Severity:  severityForPriority(e.Priority),
		Source:    "system",
		Provider:  redactText(provider),
		Message:   redactMessage(journalMessage(e.Message)),
		Redacted:  true,
	}
}

// journalMessage decodes MESSAGE, which journald emits as either a JSON string
// or an array of byte values (for non-UTF-8 messages).
func journalMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var bytes []int
	if json.Unmarshal(raw, &bytes) == nil {
		b := make([]byte, 0, len(bytes))
		for _, v := range bytes {
			b = append(b, byte(v))
		}
		return string(b)
	}
	return ""
}

func journalTimestamp(realtime string) string {
	if realtime == "" {
		return ""
	}
	micros, err := strconv.ParseInt(realtime, 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(0, micros*1000).UTC().Format(time.RFC3339)
}

func severityForPriority(p string) string {
	switch p {
	case "0", "1", "2":
		return "critical"
	case "3":
		return "error"
	case "4":
		return "warning"
	default:
		return "info"
	}
}

func linuxSummarize(logs *domain.DiagnosticsLogs) {
	providerCount := map[string]int{}
	patterns := map[string]bool{}
	for _, e := range logs.Events {
		switch e.Severity {
		case "critical":
			logs.Summary.Critical++
		case "error":
			logs.Summary.Error++
		case "warning":
			logs.Summary.Warning++
		}
		if e.Provider != "" {
			providerCount[e.Provider]++
		}
		lm := strings.ToLower(e.Message)
		switch {
		case strings.Contains(lm, "out of memory") || strings.Contains(lm, "oom-kill"):
			patterns["Out-of-memory killer activity"] = true
		case strings.Contains(lm, "i/o error") || strings.Contains(lm, "ext4-fs error") || strings.Contains(lm, "ata") && strings.Contains(lm, "error"):
			patterns["Storage / filesystem I/O errors"] = true
		case strings.Contains(lm, "thermal") || strings.Contains(lm, "throttl"):
			patterns["Thermal / throttling events"] = true
		}
	}
	logs.Summary.TopProviders = topKeys(providerCount, 5)
	logs.Summary.NotablePatterns = keysOf(patterns)
}
