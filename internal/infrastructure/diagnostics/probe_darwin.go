package diagnostics

import (
	"context"
	"os/exec"
	"strconv"
	"strings"

	"aw/internal/domain"
)

func collectProbes(_ context.Context) []domain.DiagnosticsProbeStatus {
	probes := []domain.DiagnosticsProbeStatus{
		{ID: "system.identity", Available: true},
		lookProbe("sysctl", "sysctl"),
		lookProbe("os.logs", "log"),
	}
	probes = append(probes, lookProbe("gpu.nvidia_smi", "nvidia-smi"))
	return probes
}

func lookProbe(id, bin string) domain.DiagnosticsProbeStatus {
	if _, err := exec.LookPath(bin); err == nil {
		return domain.DiagnosticsProbeStatus{ID: id, Available: true}
	}
	return domain.DiagnosticsProbeStatus{ID: id, Available: false, Reason: "not_found"}
}

func sysctlString(ctx context.Context, key string) string {
	out, err := runCommandIfPresent(ctx, "sysctl", "-n", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func sysctlInt(ctx context.Context, key string) uint64 {
	v, _ := strconv.ParseUint(sysctlString(ctx, key), 10, 64)
	return v
}

func collectSummary(ctx context.Context, _ domain.DiagnosticsSummaryOptions) (domain.DiagnosticsSummary, error) {
	s := domain.DiagnosticsSummary{
		Host:     domain.DiagnosticsHost{SKURedacted: true, HostnameRedacted: true},
		Warnings: []string{},
	}
	s.Host.Manufacturer = "Apple"
	s.Host.Model = sysctlString(ctx, "hw.model")
	name, _ := runCommandIfPresent(ctx, "sw_vers", "-productName")
	version, _ := runCommandIfPresent(ctx, "sw_vers", "-productVersion")
	s.OS.Name = nonEmpty(strings.TrimSpace(name), "macOS")
	s.OS.Version = strings.TrimSpace(version)
	s.OS.Architecture = runtimeArch()
	if boot := sysctlString(ctx, "kern.boottime"); boot != "" {
		s.OS.UptimeSecond = darwinUptime(ctx)
	}
	s.CPU = domain.DiagnosticsCPU{
		Model:         sysctlString(ctx, "machdep.cpu.brand_string"),
		LogicalCores:  int(sysctlInt(ctx, "hw.logicalcpu")),
		PhysicalCores: int(sysctlInt(ctx, "hw.physicalcpu")),
	}
	if s.CPU.LogicalCores == 0 {
		s.CPU.LogicalCores = numCPU()
	}
	total := sysctlInt(ctx, "hw.memsize")
	s.Memory = domain.DiagnosticsMemory{TotalBytes: total, UsedBytes: darwinUsedMemory(ctx, total)}
	s.Battery = darwinBattery(ctx)
	return s, nil
}

func darwinUptime(ctx context.Context) int64 {
	// sysctl -n kern.boottime → "{ sec = 171..., usec = ... }".
	raw := sysctlString(ctx, "kern.boottime")
	idx := strings.Index(raw, "sec =")
	if idx < 0 {
		return 0
	}
	rest := raw[idx+len("sec ="):]
	rest = strings.TrimSpace(rest)
	end := strings.IndexAny(rest, ", ")
	if end > 0 {
		rest = rest[:end]
	}
	bootSec, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
	if err != nil {
		return 0
	}
	now := nowUnix(ctx)
	if now > bootSec {
		return now - bootSec
	}
	return 0
}

func nowUnix(ctx context.Context) int64 {
	out, err := runCommandIfPresent(ctx, "date", "+%s")
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	return v
}

func darwinUsedMemory(ctx context.Context, total uint64) uint64 {
	// vm_stat reports page counts; used ≈ total - free - inactive.
	out, err := runCommandIfPresent(ctx, "vm_stat")
	if err != nil || out == "" || total == 0 {
		return 0
	}
	pageSize := uint64(4096)
	var freePages, inactivePages uint64
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "page size of") {
			for _, f := range strings.Fields(line) {
				if v, err := strconv.ParseUint(f, 10, 64); err == nil {
					pageSize = v
				}
			}
		}
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}
		val := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(fields[1]), "."))
		n, _ := strconv.ParseUint(val, 10, 64)
		switch {
		case strings.HasPrefix(line, "Pages free"):
			freePages = n
		case strings.HasPrefix(line, "Pages inactive"):
			inactivePages = n
		}
	}
	freeBytes := (freePages + inactivePages) * pageSize
	if freeBytes >= total {
		return 0
	}
	return total - freeBytes
}

func darwinBattery(ctx context.Context) domain.DiagnosticsBattery {
	bat := domain.DiagnosticsBattery{Status: "unknown"}
	out, err := runCommandIfPresent(ctx, "pmset", "-g", "batt")
	if err != nil || out == "" || !strings.Contains(out, "InternalBattery") {
		return bat
	}
	bat.Present = true
	if i := strings.Index(out, "%"); i > 0 {
		start := i - 1
		for start >= 0 && (out[start] >= '0' && out[start] <= '9') {
			start--
		}
		if v, err := strconv.Atoi(out[start+1 : i]); err == nil {
			bat.ChargePercent = float64(v)
		}
	}
	switch {
	case strings.Contains(out, "discharging"):
		bat.Status = "discharging"
	case strings.Contains(out, "charging"):
		bat.Status = "charging"
	case strings.Contains(out, "charged"):
		bat.Status = "full"
	}
	return bat
}

func collectGPU(_ context.Context) domain.DiagnosticsSection {
	return domain.DiagnosticsSection{
		Status:   domain.DiagnosticsStatusPartial,
		Warnings: []string{"GPU details require system_profiler SPDisplaysDataType"},
	}
}

func collectSensors(_ context.Context, _ domain.DiagnosticsSensorsOptions) (domain.DiagnosticsSensors, error) {
	return domain.DiagnosticsSensors{
		Status:       domain.DiagnosticsStatusUnsupported,
		Temperatures: []domain.DiagnosticsTemperature{},
		Fans:         []domain.DiagnosticsFan{},
		Warnings:     []string{"macOS does not expose CPU/GPU temperature or fan RPM without elevated tools; reporting unsupported rather than guessing"},
	}, nil
}

func collectStorage(ctx context.Context, opts domain.DiagnosticsStorageOptions) (domain.DiagnosticsStorage, error) {
	storage := domain.DiagnosticsStorage{
		Status:   domain.DiagnosticsStatusPartial,
		Disks:    []domain.DiagnosticsDisk{},
		Volumes:  []domain.DiagnosticsVolume{},
		Warnings: []string{},
	}
	if opts.IncludeVolumes {
		out, _ := runCommandIfPresent(ctx, "df", "-k")
		for i, line := range strings.Split(out, "\n") {
			if i == 0 || strings.TrimSpace(line) == "" {
				continue
			}
			f := strings.Fields(line)
			if len(f) < 9 || !strings.HasPrefix(f[0], "/dev/") {
				continue
			}
			size, _ := strconv.ParseUint(f[1], 10, 64)
			avail, _ := strconv.ParseUint(f[3], 10, 64)
			storage.Volumes = append(storage.Volumes, domain.DiagnosticsVolume{
				Mount: f[8], SizeBytes: size * 1024, FreeBytes: avail * 1024,
			})
		}
	}
	if len(storage.Volumes) > 0 {
		storage.Status = domain.DiagnosticsStatusOK
	}
	storage.Warnings = append(storage.Warnings, "disk model/health requires diskutil/system_profiler")
	return storage, nil
}

func collectProcesses(ctx context.Context, opts domain.DiagnosticsProcessesOptions) (domain.DiagnosticsProcesses, error) {
	result := domain.DiagnosticsProcesses{
		Status:     domain.DiagnosticsStatusOK,
		TopCPU:     []domain.DiagnosticsProcess{},
		TopMemory:  []domain.DiagnosticsProcess{},
		Redactions: []string{"commandLine", "executablePath"},
	}
	out, err := runCommandIfPresent(ctx, "ps", "-Ao", "pid=,pcpu=,rss=,comm=")
	if err != nil || out == "" {
		result.Status = domain.DiagnosticsStatusUnsupported
		result.Warnings = []string{"ps not available"}
		return result, nil
	}
	all := []domain.DiagnosticsProcess{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(f[0])
		cpu, _ := strconv.ParseFloat(f[1], 64)
		rss, _ := strconv.ParseUint(f[2], 10, 64)
		name := f[3]
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		all = append(all, domain.DiagnosticsProcess{Name: name, PID: pid, CPUPercent: cpu, MemoryBytes: rss * 1024})
	}
	result.TopCPU = topProcesses(all, opts.Limit, func(p domain.DiagnosticsProcess) float64 { return p.CPUPercent })
	result.TopMemory = topProcesses(all, opts.Limit, func(p domain.DiagnosticsProcess) float64 { return float64(p.MemoryBytes) })
	return result, nil
}

func collectDevices(_ context.Context, opts domain.DiagnosticsDevicesOptions) (domain.DiagnosticsDevices, error) {
	return domain.DiagnosticsDevices{
		Status:  domain.DiagnosticsStatusPartial,
		Mode:    opts.Mode,
		Devices: []domain.DiagnosticsDevice{},
		Summary: domain.DiagnosticsDevicesSummary{ByClass: map[string]int{}},
		CorrelationHints: []domain.DiagnosticsCorrelationHint{
			{Provider: "kernel", Reason: "check Unified Logging for extension/driver load failures"},
		},
		Warnings: []string{"detailed device inventory needs system_profiler / systemextensionsctl; correlate failures with diagnostics.logs"},
	}, nil
}

func collectLogs(_ context.Context, opts domain.DiagnosticsLogsOptions) (domain.DiagnosticsLogs, error) {
	logs := domain.DiagnosticsLogs{
		Status:   domain.DiagnosticsStatusPartial,
		Platform: platformName(),
		Events:   []domain.DiagnosticsLogEvent{},
		Warnings: []string{"bounded Unified Logging reads (log show) are not enabled in this build; use focused diagnostics.* probes instead"},
		Errors:   []string{},
	}
	logs.TimeRange.Since = opts.Since
	return logs, nil
}

func collectLogsSummary(_ context.Context, _ domain.DiagnosticsLogsSummaryOptions) (domain.DiagnosticsLogsSummary, error) {
	return domain.DiagnosticsLogsSummary{
		Status:                domain.DiagnosticsStatusPartial,
		Summary:               "macOS Unified Logging summary is not enabled in this build.",
		Patterns:              []string{},
		RecommendedNextChecks: []string{"Use diagnostics.sensors, diagnostics.storage, and diagnostics.processes for current state."},
	}, nil
}
