package domain

import (
	"strings"
	"time"
)

// DiagnosticsStatus is the normalized per-section / per-probe outcome. A missing
// sensor, unreadable log, or blocked probe is a normal partial result and must
// not fail the whole report (spec §4 decisions 5/6).
type DiagnosticsStatus string

const (
	DiagnosticsStatusOK               DiagnosticsStatus = "ok"
	DiagnosticsStatusPartial          DiagnosticsStatus = "partial"
	DiagnosticsStatusUnsupported      DiagnosticsStatus = "unsupported"
	DiagnosticsStatusPermissionDenied DiagnosticsStatus = "permission_denied"
	DiagnosticsStatusTimeout          DiagnosticsStatus = "timeout"
	DiagnosticsStatusError            DiagnosticsStatus = "error"
	DiagnosticsStatusNotInstalled     DiagnosticsStatus = "not_installed"
	DiagnosticsStatusBlocked          DiagnosticsStatus = "blocked"
)

// DiagnosticsLevel is what the current Permissions policy allows for the
// diagnostics surface. It is derived from the sandbox mode (spec §9.1):
// block_all ⇒ only diagnostics.capabilities; any other mode ⇒ detailed,
// read-only inspection with redaction.
type DiagnosticsLevel string

const (
	// DiagnosticsBlocked: only diagnostics.capabilities is available.
	DiagnosticsBlocked DiagnosticsLevel = "blocked"
	// DiagnosticsBasic: identity/summary only (reserved; not currently emitted).
	DiagnosticsBasic DiagnosticsLevel = "basic"
	// DiagnosticsDetailed: full read-only inspection with redaction.
	DiagnosticsDetailed DiagnosticsLevel = "detailed"
)

// DiagnosticsLevelForMode maps a sandbox mode to the diagnostics level. block_all
// is the only mode that blocks detailed diagnostics; every other mode allows
// read-only inspection (still redacted). This is the single source of truth used
// by both the capabilities action and the policy gate on detailed actions.
func DiagnosticsLevelForMode(mode SandboxMode) DiagnosticsLevel {
	if mode == SandboxBlockAll {
		return DiagnosticsBlocked
	}
	return DiagnosticsDetailed
}

// --- Options (parsed from action args, defaults applied in the application) ---

// DiagnosticsSummaryOptions configures diagnostics.summary.
type DiagnosticsSummaryOptions struct {
	IncludeRuntime bool
}

// DiagnosticsReportOptions configures diagnostics.report.
type DiagnosticsReportOptions struct {
	Sections  []string
	TimeoutMs int
	Redaction string
}

// DiagnosticsReportSections is the safe default set when sections is omitted
// (spec §5.3). Notably it excludes logs, which must be explicitly requested.
var DiagnosticsReportSections = []string{
	"system", "cpu", "memory", "battery", "storage", "gpu", "sensors", "processes", "devices",
}

// DiagnosticsAllReportSections is every selectable report section.
var DiagnosticsAllReportSections = append(append([]string(nil), DiagnosticsReportSections...), "logs")

// DiagnosticsSensorsOptions configures diagnostics.sensors.
type DiagnosticsSensorsOptions struct {
	TimeoutMs int
}

// DiagnosticsStorageOptions configures diagnostics.storage.
type DiagnosticsStorageOptions struct {
	IncludeHealth  bool
	IncludeVolumes bool
}

// DiagnosticsProcessesOptions configures diagnostics.processes.
type DiagnosticsProcessesOptions struct {
	SortBy   string // "cpu" | "memory"
	Limit    int
	SampleMs int
}

// DiagnosticsDevicesOptions configures diagnostics.devices.
type DiagnosticsDevicesOptions struct {
	Mode                 string // "problems" | "summary" | "all"
	IncludeDrivers       bool
	IncludeDisabled      bool
	IncludeProblemDevice bool
	Classes              []string
	Limit                int
	Redaction            string
}

// DiagnosticsDeviceClasses is the normalized cross-platform device-class filter.
var DiagnosticsDeviceClasses = []string{
	"display", "storage", "network", "battery", "usb", "bluetooth",
	"audio", "camera", "input", "system", "unknown",
}

// DiagnosticsLogsOptions configures diagnostics.logs.
type DiagnosticsLogsOptions struct {
	Since     string
	Until     string
	Severity  []string
	Sources   []string
	Query     string
	Limit     int
	Redaction string
}

// DiagnosticsLogsSummaryOptions configures diagnostics.logs.summary.
type DiagnosticsLogsSummaryOptions struct {
	Since string
	Focus string // boot|crash|storage|power|network|drivers|devices|updates|all
}

// DiagnosticsLogSources is the normalized cross-platform log-source enum. The
// backend maps each to real OS channels. "security" is opt-in and never default.
var DiagnosticsLogSources = []string{
	"system", "application", "kernel", "drivers", "devices",
	"storage", "power", "network", "updates", "security",
}

// DiagnosticsLogSeverities is the normalized severity enum.
var DiagnosticsLogSeverities = []string{"critical", "error", "warning", "info"}

// DiagnosticsSinceDurations maps the controlled `since` tokens to durations.
var DiagnosticsSinceDurations = map[string]time.Duration{
	"15m": 15 * time.Minute,
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

// ParseDiagnosticsSince resolves a controlled `since` token to a duration. An
// empty or unknown token falls back to def.
func ParseDiagnosticsSince(token string, def time.Duration) time.Duration {
	if d, ok := DiagnosticsSinceDurations[strings.TrimSpace(token)]; ok {
		return d
	}
	return def
}

// --- Result DTOs ---

// DiagnosticsActionAvailability describes one action in the capabilities report.
type DiagnosticsActionAvailability struct {
	Available bool     `json:"available"`
	Notes     []string `json:"notes,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

// DiagnosticsProbeStatus reports whether a low-level data source is present.
type DiagnosticsProbeStatus struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// DiagnosticsCapabilities is the diagnostics.capabilities response (spec §5.1).
type DiagnosticsCapabilities struct {
	Platform string `json:"platform"`
	Policy   struct {
		Mode        string `json:"mode"`
		Diagnostics string `json:"diagnostics"`
	} `json:"policy"`
	Actions map[string]DiagnosticsActionAvailability `json:"actions"`
	Probes  []DiagnosticsProbeStatus                 `json:"probes"`
}

// DiagnosticsHost is the redacted host identity (no serial/SKU/hostname).
type DiagnosticsHost struct {
	Manufacturer     string `json:"manufacturer,omitempty"`
	Model            string `json:"model,omitempty"`
	Family           string `json:"family,omitempty"`
	SKURedacted      bool   `json:"skuRedacted"`
	HostnameRedacted bool   `json:"hostnameRedacted"`
}

// DiagnosticsOS describes the operating system.
type DiagnosticsOS struct {
	Name         string `json:"name,omitempty"`
	Version      string `json:"version,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	UptimeSecond int64  `json:"uptimeSeconds,omitempty"`
}

// DiagnosticsCPU describes the processor.
type DiagnosticsCPU struct {
	Model         string  `json:"model,omitempty"`
	LogicalCores  int     `json:"logicalCores,omitempty"`
	PhysicalCores int     `json:"physicalCores,omitempty"`
	UsagePercent  float64 `json:"usagePercent,omitempty"`
}

// DiagnosticsMemory describes RAM usage.
type DiagnosticsMemory struct {
	TotalBytes uint64 `json:"totalBytes,omitempty"`
	UsedBytes  uint64 `json:"usedBytes,omitempty"`
}

// DiagnosticsBattery describes battery state.
type DiagnosticsBattery struct {
	Present       bool    `json:"present"`
	ChargePercent float64 `json:"chargePercent,omitempty"`
	Status        string  `json:"status,omitempty"` // charging|discharging|full|unknown
	HealthPercent float64 `json:"healthPercent,omitempty"`
}

// DiagnosticsSummary is the diagnostics.summary response (spec §5.2).
type DiagnosticsSummary struct {
	CollectedAt string             `json:"collectedAt"`
	Platform    string             `json:"platform"`
	Host        DiagnosticsHost    `json:"host"`
	OS          DiagnosticsOS      `json:"os"`
	CPU         DiagnosticsCPU     `json:"cpu"`
	Memory      DiagnosticsMemory  `json:"memory"`
	Battery     DiagnosticsBattery `json:"battery"`
	Warnings    []string           `json:"warnings"`
}

// DiagnosticsSection is one section of a full report with its own status.
type DiagnosticsSection struct {
	Status   DiagnosticsStatus `json:"status"`
	Data     any               `json:"data,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
	Errors   []string          `json:"errors,omitempty"`
}

// DiagnosticsReport is the diagnostics.report response (spec §5.3).
type DiagnosticsReport struct {
	CollectedAt     string                        `json:"collectedAt"`
	Platform        string                        `json:"platform"`
	Summary         *DiagnosticsSummary           `json:"summary,omitempty"`
	Sections        map[string]DiagnosticsSection `json:"sections"`
	MarkdownSummary string                        `json:"markdownSummary,omitempty"`
	Warnings        []string                      `json:"warnings"`
	Errors          []string                      `json:"errors"`
}

// DiagnosticsTemperature is one temperature reading.
type DiagnosticsTemperature struct {
	Name       string   `json:"name"`
	ValueC     *float64 `json:"valueC"`
	Source     string   `json:"source,omitempty"`
	Confidence string   `json:"confidence,omitempty"` // low|medium|high
	Status     string   `json:"status,omitempty"`
}

// DiagnosticsFan is one fan reading.
type DiagnosticsFan struct {
	Name   string `json:"name"`
	RPM    *int   `json:"rpm"`
	Status string `json:"status,omitempty"`
}

// DiagnosticsPower is current power/AC state.
type DiagnosticsPower struct {
	ACAdapterOnline   *bool    `json:"acAdapterOnline"`
	BatteryDischargeW *float64 `json:"batteryDischargeW"`
}

// DiagnosticsSensors is the diagnostics.sensors response (spec §5.4).
type DiagnosticsSensors struct {
	Status       DiagnosticsStatus        `json:"status"`
	Temperatures []DiagnosticsTemperature `json:"temperatures"`
	Fans         []DiagnosticsFan         `json:"fans"`
	Power        DiagnosticsPower         `json:"power"`
	Warnings     []string                 `json:"warnings"`
}

// DiagnosticsDisk is one physical disk (serial never returned).
type DiagnosticsDisk struct {
	Name           string   `json:"name"`
	Model          string   `json:"model,omitempty"`
	Interface      string   `json:"interface,omitempty"` // NVMe|SATA|USB|Virtual|Unknown
	SizeBytes      uint64   `json:"sizeBytes,omitempty"`
	Health         string   `json:"health,omitempty"` // OK|Warning|Critical|Unknown
	TemperatureC   *float64 `json:"temperatureC"`
	WearPercent    *float64 `json:"wearPercent"`
	SerialRedacted bool     `json:"serialRedacted"`
}

// DiagnosticsVolume is one mounted volume.
type DiagnosticsVolume struct {
	Mount      string `json:"mount"`
	Filesystem string `json:"filesystem,omitempty"`
	SizeBytes  uint64 `json:"sizeBytes,omitempty"`
	FreeBytes  uint64 `json:"freeBytes,omitempty"`
}

// DiagnosticsStorage is the diagnostics.storage response (spec §5.5).
type DiagnosticsStorage struct {
	Status   DiagnosticsStatus   `json:"status"`
	Disks    []DiagnosticsDisk   `json:"disks"`
	Volumes  []DiagnosticsVolume `json:"volumes"`
	Warnings []string            `json:"warnings"`
}

// DiagnosticsProcess is one process row (command line / path never returned).
type DiagnosticsProcess struct {
	Name        string  `json:"name"`
	PID         int     `json:"pid"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryBytes uint64  `json:"memoryBytes"`
}

// DiagnosticsProcesses is the diagnostics.processes response (spec §5.6).
type DiagnosticsProcesses struct {
	Status     DiagnosticsStatus    `json:"status"`
	TopCPU     []DiagnosticsProcess `json:"topCpu"`
	TopMemory  []DiagnosticsProcess `json:"topMemory"`
	Redactions []string             `json:"redactions"`
	Warnings   []string             `json:"warnings,omitempty"`
}

// DiagnosticsDriver describes a device's driver (paths/IDs redacted).
type DiagnosticsDriver struct {
	Provider     string `json:"provider,omitempty"`
	Version      string `json:"version,omitempty"`
	Date         string `json:"date,omitempty"`
	Signed       *bool  `json:"signed,omitempty"`
	PathRedacted bool   `json:"pathRedacted"`
}

// DiagnosticsDevice is one device with normalized status (spec §5.7).
type DiagnosticsDevice struct {
	Name             string             `json:"name"`
	Class            string             `json:"class,omitempty"`
	Status           string             `json:"status"` // ok|disabled|missing_driver|driver_error|device_error|permission_denied|unknown
	Manufacturer     string             `json:"manufacturer,omitempty"`
	Driver           *DiagnosticsDriver `json:"driver,omitempty"`
	IDsRedacted      bool               `json:"idsRedacted"`
	LocationRedacted bool               `json:"locationRedacted"`
}

// DiagnosticsCorrelationHint points the agent at log providers to correlate.
type DiagnosticsCorrelationHint struct {
	Provider string `json:"provider"`
	Reason   string `json:"reason"`
}

// DiagnosticsDevicesSummary aggregates device counts.
type DiagnosticsDevicesSummary struct {
	Total          int            `json:"total"`
	ProblemDevices int            `json:"problemDevices"`
	Disabled       int            `json:"disabled"`
	MissingDrivers int            `json:"missingDrivers"`
	ByClass        map[string]int `json:"byClass"`
}

// DiagnosticsDevices is the diagnostics.devices response (spec §5.7).
type DiagnosticsDevices struct {
	Status           DiagnosticsStatus            `json:"status"`
	Mode             string                       `json:"mode"`
	Devices          []DiagnosticsDevice          `json:"devices"`
	CorrelationHints []DiagnosticsCorrelationHint `json:"correlationHints,omitempty"`
	Summary          DiagnosticsDevicesSummary    `json:"summary"`
	Warnings         []string                     `json:"warnings"`
}

// DiagnosticsLogEvent is one redacted, truncated log event (spec §5.8).
type DiagnosticsLogEvent struct {
	Timestamp string `json:"timestamp"`
	Severity  string `json:"severity"`
	Source    string `json:"source"`
	Provider  string `json:"provider,omitempty"`
	EventID   string `json:"eventId,omitempty"`
	Message   string `json:"message"`
	Redacted  bool   `json:"redacted"`
}

// DiagnosticsLogsSummaryCounts holds severity counts.
type DiagnosticsLogsSummaryCounts struct {
	Critical int `json:"critical"`
	Error    int `json:"error"`
	Warning  int `json:"warning"`
	Info     int `json:"info,omitempty"`
}

// DiagnosticsLogs is the diagnostics.logs response (spec §5.8).
type DiagnosticsLogs struct {
	Status    DiagnosticsStatus `json:"status"`
	Platform  string            `json:"platform"`
	TimeRange struct {
		Since string `json:"since"`
		Until string `json:"until,omitempty"`
	} `json:"timeRange"`
	Events  []DiagnosticsLogEvent `json:"events"`
	Summary struct {
		Critical        int      `json:"critical"`
		Error           int      `json:"error"`
		Warning         int      `json:"warning"`
		TopProviders    []string `json:"topProviders,omitempty"`
		NotablePatterns []string `json:"notablePatterns,omitempty"`
	} `json:"summary"`
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}

// DiagnosticsLogsSummary is the diagnostics.logs.summary response (spec §5.9).
type DiagnosticsLogsSummary struct {
	Status                DiagnosticsStatus            `json:"status"`
	Summary               string                       `json:"summary"`
	Counts                DiagnosticsLogsSummaryCounts `json:"counts"`
	Patterns              []string                     `json:"patterns"`
	RecommendedNextChecks []string                     `json:"recommendedNextChecks"`
	Warnings              []string                     `json:"warnings,omitempty"`
}
