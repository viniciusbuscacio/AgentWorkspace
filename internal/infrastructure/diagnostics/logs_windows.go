package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"aw/internal/domain"
)

// winChannelForSource maps a normalized cross-platform source to a fixed Windows
// Event Log channel. Several sources collapse onto the System channel; provider
// names in each event keep them distinguishable for correlation.
func winChannelForSource(source string) string {
	switch source {
	case "application":
		return "Application"
	case "updates":
		return "Setup"
	case "security":
		return "Security"
	default: // system, kernel, drivers, devices, storage, power, network
		return "System"
	}
}

func winLevelForSeverity(sev string) int {
	switch sev {
	case "critical":
		return 1
	case "error":
		return 2
	case "warning":
		return 3
	case "info":
		return 4
	default:
		return 0
	}
}

func sourceForWinChannel(ch string) string {
	switch ch {
	case "Application":
		return "application"
	case "Setup":
		return "updates"
	case "Security":
		return "security"
	default:
		return "system"
	}
}

type winLogEventRaw struct {
	T        string `json:"t"`
	Level    int    `json:"level"`
	Channel  string `json:"channel"`
	Provider string `json:"provider"`
	ID       int    `json:"id"`
	Msg      string `json:"msg"`
}

type winLogsRaw struct {
	Events []winLogEventRaw `json:"events"`
	Errors []struct {
		Channel string `json:"channel"`
		Error   string `json:"error"`
	} `json:"errors"`
}

func collectLogs(ctx context.Context, opts domain.DiagnosticsLogsOptions) (domain.DiagnosticsLogs, error) {
	logs := domain.DiagnosticsLogs{
		Status:   domain.DiagnosticsStatusOK,
		Platform: platformName(),
		Events:   []domain.DiagnosticsLogEvent{},
		Warnings: []string{},
		Errors:   []string{},
	}
	logs.TimeRange.Since = opts.Since
	if !psAvailable() {
		logs.Status = domain.DiagnosticsStatusUnsupported
		logs.Warnings = append(logs.Warnings, "powershell not found")
		return logs, nil
	}

	channels := dedupeStrings(mapEach(opts.Sources, winChannelForSource))
	levels := dedupeInts(mapEach(opts.Severity, winLevelForSeverity))
	minutes := int(domain.ParseDiagnosticsSince(opts.Since, 0).Minutes())
	if minutes <= 0 {
		minutes = 60
	}

	out, err := runPowerShell(ctx, buildWinLogScript(channels, levels, minutes, opts.Limit))
	if err != nil {
		logs.Status = domain.DiagnosticsStatusError
		logs.Errors = append(logs.Errors, "could not read event log: "+firstLine(err.Error()))
		return logs, nil
	}
	var raw winLogsRaw
	if json.Unmarshal([]byte(strings.TrimSpace(out)), &raw) != nil {
		logs.Status = domain.DiagnosticsStatusPartial
		logs.Warnings = append(logs.Warnings, "could not parse event log output")
		return logs, nil
	}

	deniedAll := len(raw.Errors) > 0
	for _, e := range raw.Errors {
		if strings.Contains(strings.ToLower(e.Error), "no events were found") {
			deniedAll = false
			continue
		}
		if strings.Contains(strings.ToLower(e.Error), "access") || strings.Contains(strings.ToLower(e.Error), "denied") {
			logs.Warnings = append(logs.Warnings, "access denied reading the "+e.Channel+" channel (try with administrator rights)")
		} else {
			deniedAll = false
		}
	}

	query := strings.ToLower(strings.TrimSpace(opts.Query))
	for _, e := range raw.Events {
		if query != "" && !strings.Contains(strings.ToLower(e.Msg), query) {
			continue
		}
		logs.Events = append(logs.Events, domain.DiagnosticsLogEvent{
			Timestamp: e.T,
			Severity:  severityForLevel(e.Level),
			Source:    sourceForWinChannel(e.Channel),
			Provider:  redactText(e.Provider),
			EventID:   fmt.Sprintf("%d", e.ID),
			Message:   redactMessage(e.Msg),
			Redacted:  true,
		})
	}
	sort.SliceStable(logs.Events, func(i, j int) bool { return logs.Events[i].Timestamp > logs.Events[j].Timestamp })
	if len(logs.Events) > opts.Limit {
		logs.Events = logs.Events[:opts.Limit]
	}

	summarizeWinEvents(&logs)
	if len(logs.Events) == 0 && deniedAll {
		logs.Status = domain.DiagnosticsStatusPermissionDenied
	} else if len(logs.Warnings) > 0 {
		logs.Status = domain.DiagnosticsStatusPartial
	}
	return logs, nil
}

func buildWinLogScript(channels []string, levels []int, minutes, perChannel int) string {
	chList := psStringArray(channels)
	lvList := psIntArray(levels)
	return fmt.Sprintf(`
$ErrorActionPreference='SilentlyContinue'
$lv=%s
$start=(Get-Date).AddMinutes(-%d)
$events=@(); $errs=@()
foreach($ch in %s){
  try {
    $ev=Get-WinEvent -FilterHashtable @{ LogName=$ch; Level=$lv; StartTime=$start } -MaxEvents %d -ErrorAction Stop
    foreach($e in $ev){
      $events += [pscustomobject]@{ t=$e.TimeCreated.ToUniversalTime().ToString('o'); level=[int]$e.Level; channel=$ch; provider=[string]$e.ProviderName; id=[int]$e.Id; msg=[string]$e.Message }
    }
  } catch {
    $errs += [pscustomobject]@{ channel=$ch; error=[string]$_.Exception.Message }
  }
}
[pscustomobject]@{ events=$events; errors=$errs } | ConvertTo-Json -Compress -Depth 4`,
		lvList, minutes, chList, perChannel)
}

func summarizeWinEvents(logs *domain.DiagnosticsLogs) {
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
		lp := strings.ToLower(e.Provider)
		if strings.Contains(lp, "kernel-power") && e.EventID == "41" {
			patterns["Unexpected shutdown / power loss (Kernel-Power 41)"] = true
		}
		if strings.Contains(lp, "bugcheck") || e.EventID == "1001" && strings.Contains(lp, "windows error reporting") {
			patterns["Possible system crash (BugCheck)"] = true
		}
		if strings.Contains(lp, "disk") || strings.Contains(lp, "ntfs") || strings.Contains(lp, "stornvme") || strings.Contains(lp, "storahci") {
			patterns["Storage subsystem warnings/errors"] = true
		}
		if strings.Contains(lp, "whea") {
			patterns["Hardware error reported (WHEA)"] = true
		}
	}
	logs.Summary.TopProviders = topKeys(providerCount, 5)
	logs.Summary.NotablePatterns = keysOf(patterns)
}

func collectLogsSummary(ctx context.Context, opts domain.DiagnosticsLogsSummaryOptions) (domain.DiagnosticsLogsSummary, error) {
	logOpts := logsOptionsForFocus(opts)
	detail, _ := collectLogs(ctx, logOpts)
	summary := domain.DiagnosticsLogsSummary{
		Status: detail.Status,
		Counts: domain.DiagnosticsLogsSummaryCounts{
			Critical: detail.Summary.Critical,
			Error:    detail.Summary.Error,
			Warning:  detail.Summary.Warning,
		},
		Patterns:              detail.Summary.NotablePatterns,
		RecommendedNextChecks: recommendedChecks(opts.Focus, detail),
		Warnings:              detail.Warnings,
	}
	if summary.Patterns == nil {
		summary.Patterns = []string{}
	}
	total := summary.Counts.Critical + summary.Counts.Error + summary.Counts.Warning
	if total == 0 {
		summary.Summary = fmt.Sprintf("No critical/error/warning events in the last %s for focus %q.", opts.Since, opts.Focus)
	} else {
		summary.Summary = fmt.Sprintf("%d notable events in the last %s (%d critical, %d error, %d warning). Top providers: %s.",
			total, opts.Since, summary.Counts.Critical, summary.Counts.Error, summary.Counts.Warning,
			strings.Join(detail.Summary.TopProviders, ", "))
	}
	return summary, nil
}

func logsOptionsForFocus(opts domain.DiagnosticsLogsSummaryOptions) domain.DiagnosticsLogsOptions {
	sources := []string{"system", "application", "kernel", "storage", "power", "drivers", "devices"}
	switch opts.Focus {
	case "boot", "crash", "power":
		sources = []string{"system", "power", "kernel"}
	case "storage":
		sources = []string{"system", "storage"}
	case "network":
		sources = []string{"system", "network"}
	case "drivers", "devices":
		sources = []string{"system", "drivers", "devices"}
	case "updates":
		sources = []string{"updates", "application"}
	}
	return domain.DiagnosticsLogsOptions{
		Since:    opts.Since,
		Severity: []string{"critical", "error", "warning"},
		Sources:  sources,
		Limit:    200,
	}
}

func recommendedChecks(_ string, detail domain.DiagnosticsLogs) []string {
	checks := []string{}
	for _, p := range detail.Summary.NotablePatterns {
		switch {
		case strings.Contains(p, "Kernel-Power"):
			checks = append(checks, "Investigate unexpected shutdowns: check power settings, battery health (diagnostics.sensors), and overheating.")
		case strings.Contains(p, "Storage"):
			checks = append(checks, "Run diagnostics.storage to check disk health and free space.")
		case strings.Contains(p, "WHEA"):
			checks = append(checks, "Hardware errors detected; check diagnostics.sensors and diagnostics.devices for failing components.")
		case strings.Contains(p, "BugCheck"):
			checks = append(checks, "System crash detected; review diagnostics.logs with focus=crash for the bugcheck code.")
		}
	}
	if len(checks) == 0 {
		checks = append(checks, "No urgent issues found; re-run with a longer 'since' window if a problem is intermittent.")
	}
	return dedupeStrings(checks)
}

// --- small helpers ---

func severityForLevel(level int) string {
	switch level {
	case 1:
		return "critical"
	case 2:
		return "error"
	case 3:
		return "warning"
	default:
		return "info"
	}
}

func mapEach[T any, R any](in []T, f func(T) R) []R {
	out := make([]R, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func dedupeInts(in []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(in))
	for _, v := range in {
		if v != 0 && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func psStringArray(in []string) string {
	quoted := make([]string, len(in))
	for i, s := range in {
		quoted[i] = "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return "@(" + strings.Join(quoted, ",") + ")"
}

func psIntArray(in []int) string {
	parts := make([]string, len(in))
	for i, n := range in {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return "@(" + strings.Join(parts, ",") + ")"
}
