package diagnostics

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"aw/internal/domain"
)

func collectProbes(_ context.Context) []domain.DiagnosticsProbeStatus {
	probes := []domain.DiagnosticsProbeStatus{
		{ID: "system.identity", Available: true},
		{ID: "proc.fs", Available: fileExists("/proc/meminfo")},
		{ID: "sys.thermal", Available: dirHasEntries("/sys/class/thermal")},
	}
	probes = append(probes,
		lookProbe("os.logs", "journalctl"),
		lookProbe("storage.lsblk", "lsblk"),
		lookProbe("sensors.lm_sensors", "sensors"),
		lookProbe("gpu.nvidia_smi", "nvidia-smi"),
	)
	return probes
}

func lookProbe(id, bin string) domain.DiagnosticsProbeStatus {
	if _, err := exec.LookPath(bin); err == nil {
		return domain.DiagnosticsProbeStatus{ID: id, Available: true}
	}
	return domain.DiagnosticsProbeStatus{ID: id, Available: false, Reason: "not_found"}
}

func collectSummary(_ context.Context, _ domain.DiagnosticsSummaryOptions) (domain.DiagnosticsSummary, error) {
	s := domain.DiagnosticsSummary{
		Host:     domain.DiagnosticsHost{SKURedacted: true, HostnameRedacted: true},
		Warnings: []string{},
	}
	// OS identity.
	osRelease := parseKeyVals(readFileString("/etc/os-release"))
	s.OS.Name = unquote(firstNonEmpty(osRelease["PRETTY_NAME"], osRelease["NAME"], "Linux"))
	s.OS.Version = strings.TrimSpace(readFileString("/proc/sys/kernel/osrelease"))
	s.OS.Architecture = runtimeArch()
	if up := readFileString("/proc/uptime"); up != "" {
		if f := strings.Fields(up); len(f) > 0 {
			if v, err := strconv.ParseFloat(f[0], 64); err == nil {
				s.OS.UptimeSecond = int64(v)
			}
		}
	}
	// Host (DMI), serials never read.
	s.Host.Manufacturer = strings.TrimSpace(readFileString("/sys/class/dmi/id/sys_vendor"))
	s.Host.Model = strings.TrimSpace(readFileString("/sys/class/dmi/id/product_name"))
	s.Host.Family = strings.TrimSpace(readFileString("/sys/class/dmi/id/product_family"))
	// CPU.
	s.CPU = linuxCPU()
	// Memory.
	mem := parseMeminfo(readFileString("/proc/meminfo"))
	s.Memory.TotalBytes = mem["MemTotal"] * 1024
	if avail, ok := mem["MemAvailable"]; ok && mem["MemTotal"] >= avail {
		s.Memory.UsedBytes = (mem["MemTotal"] - avail) * 1024
	}
	// Battery.
	s.Battery = linuxBattery()
	if s.OS.Name == "Linux" && s.Host.Manufacturer == "" {
		s.Warnings = append(s.Warnings, "limited host identity (DMI not readable)")
	}
	return s, nil
}

func linuxCPU() domain.DiagnosticsCPU {
	cpu := domain.DiagnosticsCPU{LogicalCores: numCPU()}
	info := readFileString("/proc/cpuinfo")
	coreIDs := map[string]bool{}
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, "model name") && cpu.Model == "" {
			if i := strings.Index(line, ":"); i >= 0 {
				cpu.Model = strings.TrimSpace(line[i+1:])
			}
		}
		if strings.HasPrefix(line, "core id") {
			if i := strings.Index(line, ":"); i >= 0 {
				coreIDs[strings.TrimSpace(line[i+1:])] = true
			}
		}
	}
	if len(coreIDs) > 0 {
		cpu.PhysicalCores = len(coreIDs)
	}
	return cpu
}

func linuxBattery() domain.DiagnosticsBattery {
	bat := domain.DiagnosticsBattery{Status: "unknown"}
	base := "/sys/class/power_supply"
	entries, err := os.ReadDir(base)
	if err != nil {
		return bat
	}
	for _, e := range entries {
		typ := strings.TrimSpace(readFileString(filepath.Join(base, e.Name(), "type")))
		if !strings.EqualFold(typ, "Battery") {
			continue
		}
		bat.Present = true
		if cap := strings.TrimSpace(readFileString(filepath.Join(base, e.Name(), "capacity"))); cap != "" {
			if v, err := strconv.Atoi(cap); err == nil {
				bat.ChargePercent = float64(v)
			}
		}
		switch strings.ToLower(strings.TrimSpace(readFileString(filepath.Join(base, e.Name(), "status")))) {
		case "charging":
			bat.Status = "charging"
		case "discharging":
			bat.Status = "discharging"
		case "full":
			bat.Status = "full"
		}
		break
	}
	return bat
}

func collectGPU(ctx context.Context) domain.DiagnosticsSection {
	// /sys/class/drm exposes GPU cards without privilege; names need lspci.
	if out, err := runCommandIfPresent(ctx, "lspci", "-mm"); err == nil && out != "" {
		gpus := []map[string]any{}
		for _, line := range strings.Split(out, "\n") {
			low := strings.ToLower(line)
			if strings.Contains(low, "vga") || strings.Contains(low, "3d controller") || strings.Contains(low, "display controller") {
				gpus = append(gpus, map[string]any{"name": redactText(strings.TrimSpace(line))})
			}
		}
		if len(gpus) > 0 {
			return okSection(map[string]any{"gpus": gpus})
		}
	}
	return domain.DiagnosticsSection{Status: domain.DiagnosticsStatusPartial, Warnings: []string{"GPU enumeration needs lspci"}}
}

func collectSensors(_ context.Context, _ domain.DiagnosticsSensorsOptions) (domain.DiagnosticsSensors, error) {
	sensors := domain.DiagnosticsSensors{
		Status:       domain.DiagnosticsStatusUnsupported,
		Temperatures: []domain.DiagnosticsTemperature{},
		Fans:         []domain.DiagnosticsFan{},
		Warnings:     []string{},
	}
	base := "/sys/class/thermal"
	entries, _ := os.ReadDir(base)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "thermal_zone") {
			continue
		}
		raw := strings.TrimSpace(readFileString(filepath.Join(base, e.Name(), "temp")))
		if raw == "" {
			continue
		}
		milli, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		c := milli / 1000.0
		if c <= 0 || c > 150 {
			continue
		}
		name := strings.TrimSpace(readFileString(filepath.Join(base, e.Name(), "type")))
		if name == "" {
			name = e.Name()
		}
		sensors.Temperatures = append(sensors.Temperatures, domain.DiagnosticsTemperature{
			Name: name, ValueC: floatPtr(roundTo(c, 1)), Source: "sysfs:thermal", Confidence: "medium", Status: "ok",
		})
	}
	sensors.Power = linuxPower()
	sensors.Fans = append(sensors.Fans, domain.DiagnosticsFan{Name: "fan0", RPM: nil, Status: "unsupported"})
	if len(sensors.Temperatures) > 0 {
		sensors.Status = domain.DiagnosticsStatusPartial
	} else {
		sensors.Warnings = append(sensors.Warnings, "no thermal zones exposed under /sys/class/thermal; install lm-sensors for richer data")
	}
	return sensors, nil
}

func linuxPower() domain.DiagnosticsPower {
	power := domain.DiagnosticsPower{}
	base := "/sys/class/power_supply"
	entries, _ := os.ReadDir(base)
	for _, e := range entries {
		typ := strings.TrimSpace(readFileString(filepath.Join(base, e.Name(), "type")))
		if strings.EqualFold(typ, "Mains") {
			if online := strings.TrimSpace(readFileString(filepath.Join(base, e.Name(), "online"))); online != "" {
				power.ACAdapterOnline = boolPtr(online == "1")
			}
		}
	}
	return power
}

func collectStorage(_ context.Context, opts domain.DiagnosticsStorageOptions) (domain.DiagnosticsStorage, error) {
	storage := domain.DiagnosticsStorage{
		Status:   domain.DiagnosticsStatusPartial,
		Disks:    []domain.DiagnosticsDisk{},
		Volumes:  []domain.DiagnosticsVolume{},
		Warnings: []string{},
	}
	if opts.IncludeVolumes {
		for _, m := range parseMounts() {
			var st syscall.Statfs_t
			if syscall.Statfs(m.mount, &st) != nil {
				continue
			}
			bs := uint64(st.Bsize)
			storage.Volumes = append(storage.Volumes, domain.DiagnosticsVolume{
				Mount: m.mount, Filesystem: m.fstype,
				SizeBytes: st.Blocks * bs, FreeBytes: st.Bavail * bs,
			})
		}
	}
	if len(storage.Volumes) > 0 {
		storage.Status = domain.DiagnosticsStatusOK
	}
	storage.Warnings = append(storage.Warnings, "disk model/health requires lsblk/smartctl and may need privileges")
	return storage, nil
}

type linuxMount struct{ mount, fstype string }

func parseMounts() []linuxMount {
	out := []linuxMount{}
	realFS := map[string]bool{"ext4": true, "ext3": true, "xfs": true, "btrfs": true, "vfat": true, "ntfs": true, "f2fs": true, "zfs": true, "exfat": true}
	for _, line := range strings.Split(readFileString("/proc/mounts"), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || !realFS[f[2]] {
			continue
		}
		out = append(out, linuxMount{mount: f[1], fstype: f[2]})
	}
	return out
}

func collectProcesses(ctx context.Context, opts domain.DiagnosticsProcessesOptions) (domain.DiagnosticsProcesses, error) {
	result := domain.DiagnosticsProcesses{
		Status:     domain.DiagnosticsStatusOK,
		TopCPU:     []domain.DiagnosticsProcess{},
		TopMemory:  []domain.DiagnosticsProcess{},
		Redactions: []string{"commandLine", "executablePath"},
	}
	first := snapshotProcCPU()
	sample := opts.SampleMs
	if sample <= 0 {
		sample = 500
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Duration(sample) * time.Millisecond):
	}
	second := snapshotProcCPU()
	pageSize := uint64(os.Getpagesize())
	clk := 100.0 // USER_HZ; standard on Linux
	n := float64(numCPU())
	secs := float64(sample) / 1000.0
	all := []domain.DiagnosticsProcess{}
	for pid, s2 := range second {
		s1, ok := first[pid]
		cpu := 0.0
		if ok && s2.ticks >= s1.ticks {
			cpu = float64(s2.ticks-s1.ticks) / clk / secs / n * 100
		}
		all = append(all, domain.DiagnosticsProcess{Name: s2.name, PID: pid, CPUPercent: roundTo(cpu, 1), MemoryBytes: s2.rss * pageSize})
	}
	result.TopCPU = topProcesses(all, opts.Limit, func(p domain.DiagnosticsProcess) float64 { return p.CPUPercent })
	result.TopMemory = topProcesses(all, opts.Limit, func(p domain.DiagnosticsProcess) float64 { return float64(p.MemoryBytes) })
	return result, nil
}

type procStat struct {
	name  string
	ticks uint64
	rss   uint64
}

func snapshotProcCPU() map[int]procStat {
	out := map[int]procStat{}
	entries, _ := os.ReadDir("/proc")
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat := readFileString(filepath.Join("/proc", e.Name(), "stat"))
		if stat == "" {
			continue
		}
		// comm is in parens and may contain spaces; split after the closing paren.
		close := strings.LastIndex(stat, ")")
		open := strings.Index(stat, "(")
		if open < 0 || close < 0 || close < open {
			continue
		}
		name := stat[open+1 : close]
		fields := strings.Fields(stat[close+1:])
		// after ")" fields[0]=state; utime=field index 11, stime=12, rss=21 (0-based from state).
		if len(fields) < 22 {
			continue
		}
		utime, _ := strconv.ParseUint(fields[11], 10, 64)
		stime, _ := strconv.ParseUint(fields[12], 10, 64)
		rss, _ := strconv.ParseUint(fields[21], 10, 64)
		out[pid] = procStat{name: name, ticks: utime + stime, rss: rss}
	}
	return out
}

func collectDevices(_ context.Context, opts domain.DiagnosticsDevicesOptions) (domain.DiagnosticsDevices, error) {
	return domain.DiagnosticsDevices{
		Status:  domain.DiagnosticsStatusPartial,
		Mode:    opts.Mode,
		Devices: []domain.DiagnosticsDevice{},
		Summary: domain.DiagnosticsDevicesSummary{ByClass: map[string]int{}},
		CorrelationHints: []domain.DiagnosticsCorrelationHint{
			{Provider: "kernel", Reason: "check dmesg / journal kernel messages for driver/module load failures"},
		},
		Warnings: []string{"detailed device/driver inventory needs lspci -k / lsusb; correlate failures with diagnostics.logs"},
	}, nil
}

func collectLogs(ctx context.Context, opts domain.DiagnosticsLogsOptions) (domain.DiagnosticsLogs, error) {
	logs := domain.DiagnosticsLogs{
		Status:   domain.DiagnosticsStatusUnsupported,
		Platform: platformName(),
		Events:   []domain.DiagnosticsLogEvent{},
		Warnings: []string{},
		Errors:   []string{},
	}
	logs.TimeRange.Since = opts.Since
	if _, err := exec.LookPath("journalctl"); err != nil {
		logs.Warnings = append(logs.Warnings, "journalctl not found; OS log reading is unavailable")
		return logs, nil
	}
	// security source is opt-in; journald has no separate auth journal we read here.
	since := journalSince(opts.Since)
	priority := journalPriority(opts.Severity)
	out, err := runCommand(ctx, "journalctl", "--no-pager", "-o", "json", "--since", since, "-p", priority, "-n", strconv.Itoa(opts.Limit))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "permission") {
			logs.Status = domain.DiagnosticsStatusPermissionDenied
			logs.Warnings = append(logs.Warnings, "permission denied reading the journal (user not in systemd-journal group)")
			return logs, nil
		}
		logs.Status = domain.DiagnosticsStatusError
		logs.Errors = append(logs.Errors, firstLine(err.Error()))
		return logs, nil
	}
	query := strings.ToLower(strings.TrimSpace(opts.Query))
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ev := parseJournalLine(line)
		if ev.Message == "" {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(ev.Message), query) {
			continue
		}
		logs.Events = append(logs.Events, ev)
	}
	logs.Status = domain.DiagnosticsStatusOK
	linuxSummarize(&logs)
	return logs, nil
}

func collectLogsSummary(ctx context.Context, opts domain.DiagnosticsLogsSummaryOptions) (domain.DiagnosticsLogsSummary, error) {
	detail, _ := collectLogs(ctx, domain.DiagnosticsLogsOptions{
		Since: opts.Since, Severity: []string{"critical", "error", "warning"},
		Sources: []string{"system", "kernel"}, Limit: 200,
	})
	summary := domain.DiagnosticsLogsSummary{
		Status:                detail.Status,
		Counts:                domain.DiagnosticsLogsSummaryCounts{Critical: detail.Summary.Critical, Error: detail.Summary.Error, Warning: detail.Summary.Warning},
		Patterns:              detail.Summary.NotablePatterns,
		RecommendedNextChecks: []string{},
		Warnings:              detail.Warnings,
	}
	if summary.Patterns == nil {
		summary.Patterns = []string{}
	}
	total := summary.Counts.Critical + summary.Counts.Error + summary.Counts.Warning
	if total == 0 {
		summary.Summary = "No notable journal events in the requested window."
	} else {
		summary.Summary = strings.TrimSpace(strings.Join(detail.Summary.TopProviders, ", "))
		summary.Summary = "Notable journal events found. Top units: " + summary.Summary
	}
	if total == 0 {
		summary.RecommendedNextChecks = append(summary.RecommendedNextChecks, "Re-run with a longer 'since' window if a problem is intermittent.")
	}
	return summary, nil
}
