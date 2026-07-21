package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"aw/internal/domain"
)

// runPowerShell runs a FIXED PowerShell script bounded by ctx. The script is a
// compile-time constant (or built only from validated enums/ints). The model
// never supplies any part of it. -NoProfile/-NonInteractive keep it hermetic.
func runPowerShell(ctx context.Context, script string) (string, error) {
	return runCommand(ctx, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
}

// psAvailable reports whether powershell is on PATH (it always is on supported
// Windows, but we degrade honestly if not).
func psAvailable() bool {
	_, err := exec.LookPath("powershell")
	return err == nil
}

func collectProbes(_ context.Context) []domain.DiagnosticsProbeStatus {
	probes := []domain.DiagnosticsProbeStatus{
		{ID: "system.identity", Available: true},
	}
	ps := psAvailable()
	probes = append(probes,
		domain.DiagnosticsProbeStatus{ID: "cim.wmi", Available: ps, Reason: reasonIf(!ps, "powershell_not_found")},
		domain.DiagnosticsProbeStatus{ID: "os.logs", Available: ps, Reason: reasonIf(!ps, "powershell_not_found")},
	)
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		probes = append(probes, domain.DiagnosticsProbeStatus{ID: "gpu.nvidia_smi", Available: true})
	} else {
		probes = append(probes, domain.DiagnosticsProbeStatus{ID: "gpu.nvidia_smi", Available: false, Reason: "not_found"})
	}
	return probes
}

func reasonIf(cond bool, reason string) string {
	if cond {
		return reason
	}
	return ""
}

// --- summary ---

type winSummaryRaw struct {
	OSName       string `json:"osName"`
	OSVersion    string `json:"osVersion"`
	Arch         string `json:"arch"`
	Uptime       int64  `json:"uptime"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Family       string `json:"family"`
	TotalMem     uint64 `json:"totalMem"`
	FreeMemKB    uint64 `json:"freeMemKB"`
	TotalMemKB   uint64 `json:"totalMemKB"`
	CPUName      string `json:"cpuName"`
	Cores        int    `json:"cores"`
	Logical      int    `json:"logical"`
	Load         *int   `json:"load"`
	BatPresent   bool   `json:"batPresent"`
	BatCharge    *int   `json:"batCharge"`
	BatStatus    *int   `json:"batStatus"`
}

const winSummaryScript = `
$ErrorActionPreference='SilentlyContinue'
$os=Get-CimInstance Win32_OperatingSystem
$cs=Get-CimInstance Win32_ComputerSystem
$cpu=Get-CimInstance Win32_Processor | Select-Object -First 1
$bat=Get-CimInstance Win32_Battery | Select-Object -First 1
$up=0; if($os.LastBootUpTime){ $up=[int64]((Get-Date)-$os.LastBootUpTime).TotalSeconds }
[pscustomobject]@{
 osName=$os.Caption; osVersion=$os.Version; arch=$os.OSArchitecture; uptime=$up;
 manufacturer=$cs.Manufacturer; model=$cs.Model; family=$cs.SystemFamily;
 totalMem=[uint64]$cs.TotalPhysicalMemory; freeMemKB=[uint64]$os.FreePhysicalMemory; totalMemKB=[uint64]$os.TotalVisibleMemorySize;
 cpuName=$cpu.Name; cores=[int]$cpu.NumberOfCores; logical=[int]$cpu.NumberOfLogicalProcessors; load=$cpu.LoadPercentage;
 batPresent=[bool]$bat; batCharge=$bat.EstimatedChargeRemaining; batStatus=$bat.BatteryStatus
} | ConvertTo-Json -Compress`

func collectSummary(ctx context.Context, _ domain.DiagnosticsSummaryOptions) (domain.DiagnosticsSummary, error) {
	summary := domain.DiagnosticsSummary{
		Host:     domain.DiagnosticsHost{SKURedacted: true, HostnameRedacted: true},
		Warnings: []string{},
	}
	if !psAvailable() {
		summary.Warnings = append(summary.Warnings, "powershell not found; summary is limited")
		return summary, nil
	}
	out, err := runPowerShell(ctx, winSummaryScript)
	if err != nil {
		summary.Warnings = append(summary.Warnings, "could not read system summary: "+firstLine(err.Error()))
		return summary, nil
	}
	var raw winSummaryRaw
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &raw); err != nil {
		summary.Warnings = append(summary.Warnings, "could not parse system summary")
		return summary, nil
	}
	summary.Host.Manufacturer = strings.TrimSpace(raw.Manufacturer)
	summary.Host.Model = strings.TrimSpace(raw.Model)
	summary.Host.Family = strings.TrimSpace(raw.Family)
	summary.OS = domain.DiagnosticsOS{Name: raw.OSName, Version: raw.OSVersion, Architecture: raw.Arch, UptimeSecond: raw.Uptime}
	summary.CPU = domain.DiagnosticsCPU{Model: strings.TrimSpace(raw.CPUName), LogicalCores: raw.Logical, PhysicalCores: raw.Cores}
	if raw.Load != nil {
		summary.CPU.UsagePercent = float64(*raw.Load)
	}
	total := raw.TotalMem
	if total == 0 {
		total = raw.TotalMemKB * 1024
	}
	summary.Memory = domain.DiagnosticsMemory{TotalBytes: total}
	if raw.TotalMemKB >= raw.FreeMemKB {
		summary.Memory.UsedBytes = (raw.TotalMemKB - raw.FreeMemKB) * 1024
	}
	summary.Battery = domain.DiagnosticsBattery{Present: raw.BatPresent, Status: "unknown"}
	if raw.BatCharge != nil {
		summary.Battery.ChargePercent = float64(*raw.BatCharge)
	}
	if raw.BatStatus != nil {
		summary.Battery.Status = winBatteryStatus(*raw.BatStatus)
	}
	return summary, nil
}

func winBatteryStatus(code int) string {
	switch code {
	case 1, 4, 5, 11:
		return "discharging"
	case 3:
		return "full"
	case 6, 7, 8, 9:
		return "charging"
	default:
		return "unknown"
	}
}

// --- gpu ---

type winGPURaw struct {
	Name          string `json:"name"`
	DriverVersion string `json:"driver"`
	AdapterRAM    int64  `json:"vram"`
}

const winGPUScript = `
$ErrorActionPreference='SilentlyContinue'
@(Get-CimInstance Win32_VideoController | ForEach-Object {
  [pscustomobject]@{ name=$_.Name; driver=$_.DriverVersion; vram=[int64]$_.AdapterRAM }
}) | ConvertTo-Json -Compress`

func collectGPU(ctx context.Context) domain.DiagnosticsSection {
	if !psAvailable() {
		return domain.DiagnosticsSection{Status: domain.DiagnosticsStatusUnsupported}
	}
	out, err := runPowerShell(ctx, winGPUScript)
	if err != nil {
		return domain.DiagnosticsSection{Status: domain.DiagnosticsStatusError, Errors: []string{firstLine(err.Error())}}
	}
	raws := unmarshalArray[winGPURaw](out)
	gpus := make([]map[string]any, 0, len(raws))
	for _, g := range raws {
		entry := map[string]any{"name": strings.TrimSpace(g.Name)}
		if g.DriverVersion != "" {
			entry["driverVersion"] = g.DriverVersion
		}
		if g.AdapterRAM > 0 {
			entry["vramBytes"] = g.AdapterRAM
		}
		gpus = append(gpus, entry)
	}
	if len(gpus) == 0 {
		return domain.DiagnosticsSection{Status: domain.DiagnosticsStatusPartial, Data: map[string]any{"gpus": gpus}}
	}
	return okSection(map[string]any{"gpus": gpus})
}

// --- processes ---

type winProcRaw struct {
	PID  int     `json:"pid"`
	Name string  `json:"name"`
	CPU  float64 `json:"cpu"`
	Mem  uint64  `json:"mem"`
}

func collectProcesses(ctx context.Context, opts domain.DiagnosticsProcessesOptions) (domain.DiagnosticsProcesses, error) {
	result := domain.DiagnosticsProcesses{
		Status:     domain.DiagnosticsStatusOK,
		TopCPU:     []domain.DiagnosticsProcess{},
		TopMemory:  []domain.DiagnosticsProcess{},
		Redactions: []string{"commandLine", "executablePath"},
	}
	if !psAvailable() {
		result.Status = domain.DiagnosticsStatusUnsupported
		result.Warnings = []string{"powershell not found"}
		return result, nil
	}
	sample := opts.SampleMs
	if sample <= 0 {
		sample = 500
	}
	// CPU% is computed from the delta of each process's total processor time over
	// a short bounded sample window (spec allows brief internal waits).
	script := fmt.Sprintf(`
$ErrorActionPreference='SilentlyContinue'
$n=[Environment]::ProcessorCount
$s1=Get-Process | Select-Object Id,ProcessName,CPU
$m=@{}; foreach($p in $s1){ $m[$p.Id]=$p.CPU }
Start-Sleep -Milliseconds %d
$s2=Get-Process | Select-Object Id,ProcessName,CPU,WorkingSet64
@(foreach($p in $s2){
  $prev=$m[$p.Id]; $d=0.0; if($prev -ne $null){ $d=$p.CPU-$prev }
  $pct=0.0; if($d -gt 0){ $pct=[math]::Round(($d/(%d/1000.0)/$n*100),1) }
  [pscustomobject]@{ pid=$p.Id; name=$p.ProcessName; cpu=$pct; mem=[uint64]$p.WorkingSet64 }
}) | ConvertTo-Json -Compress`, sample, sample)
	out, err := runPowerShell(ctx, script)
	if err != nil {
		result.Status = domain.DiagnosticsStatusError
		result.Warnings = []string{"could not read processes: " + firstLine(err.Error())}
		return result, nil
	}
	raws := unmarshalArray[winProcRaw](out)
	all := make([]domain.DiagnosticsProcess, 0, len(raws))
	for _, r := range raws {
		all = append(all, domain.DiagnosticsProcess{Name: r.Name, PID: r.PID, CPUPercent: r.CPU, MemoryBytes: r.Mem})
	}
	result.TopCPU = topProcesses(all, opts.Limit, func(p domain.DiagnosticsProcess) float64 { return p.CPUPercent })
	result.TopMemory = topProcesses(all, opts.Limit, func(p domain.DiagnosticsProcess) float64 { return float64(p.MemoryBytes) })
	return result, nil
}

// --- sensors ---

const winThermalScript = `
$ErrorActionPreference='SilentlyContinue'
@(Get-CimInstance -Namespace root/WMI -ClassName MSAcpi_ThermalZoneTemperature -ErrorAction SilentlyContinue | ForEach-Object {
  [pscustomobject]@{ name=$_.InstanceName; temp=[double]$_.CurrentTemperature }
}) | ConvertTo-Json -Compress`

type winThermalRaw struct {
	Name string  `json:"name"`
	Temp float64 `json:"temp"`
}

func collectSensors(ctx context.Context, _ domain.DiagnosticsSensorsOptions) (domain.DiagnosticsSensors, error) {
	sensors := domain.DiagnosticsSensors{
		Status:       domain.DiagnosticsStatusUnsupported,
		Temperatures: []domain.DiagnosticsTemperature{},
		Fans:         []domain.DiagnosticsFan{},
		Warnings:     []string{},
	}
	if !psAvailable() {
		sensors.Warnings = append(sensors.Warnings, "powershell not found")
		return sensors, nil
	}
	out, err := runPowerShell(ctx, winThermalScript)
	if err == nil {
		for _, t := range unmarshalArray[winThermalRaw](out) {
			if t.Temp <= 0 {
				continue
			}
			c := t.Temp/10.0 - 273.15
			if c < -50 || c > 150 {
				continue
			}
			sensors.Temperatures = append(sensors.Temperatures, domain.DiagnosticsTemperature{
				Name:       "thermal_zone",
				ValueC:     floatPtr(roundTo(c, 1)),
				Source:     "MSAcpi_ThermalZoneTemperature",
				Confidence: "low",
				Status:     "ok",
			})
		}
	}
	// AC adapter / battery power state is reliable; fan RPM is not exposed by
	// Windows without vendor drivers.
	sensors.Power = winPowerState(ctx)
	sensors.Fans = append(sensors.Fans, domain.DiagnosticsFan{Name: "fan0", RPM: nil, Status: "unsupported"})
	if len(sensors.Temperatures) > 0 {
		sensors.Status = domain.DiagnosticsStatusPartial
	} else {
		sensors.Warnings = append(sensors.Warnings, "no thermal sensor exposed by ACPI on this machine; CPU/GPU temperature is unavailable without a vendor sensor provider")
	}
	sensors.Warnings = append(sensors.Warnings, "fan RPM is not exposed by Windows without vendor drivers")
	return sensors, nil
}

const winPowerScript = `
$ErrorActionPreference='SilentlyContinue'
$b=Get-CimInstance Win32_Battery | Select-Object -First 1
$online=$null; if($b){ $online=($b.BatteryStatus -ne 1) }
[pscustomobject]@{ present=[bool]$b; online=$online } | ConvertTo-Json -Compress`

type winPowerRaw struct {
	Present bool  `json:"present"`
	Online  *bool `json:"online"`
}

func winPowerState(ctx context.Context) domain.DiagnosticsPower {
	power := domain.DiagnosticsPower{}
	out, err := runPowerShell(ctx, winPowerScript)
	if err != nil {
		return power
	}
	var raw winPowerRaw
	if json.Unmarshal([]byte(strings.TrimSpace(out)), &raw) == nil {
		if !raw.Present {
			online := true // no battery ⇒ desktop on AC
			power.ACAdapterOnline = &online
		} else {
			power.ACAdapterOnline = raw.Online
		}
	}
	return power
}

// --- storage ---

type winDiskRaw struct {
	Name   string `json:"name"`
	Media  string `json:"media"`
	Bus    string `json:"bus"`
	Size   uint64 `json:"size"`
	Health string `json:"health"`
}

type winVolRaw struct {
	Letter string `json:"letter"`
	FS     string `json:"fs"`
	Size   uint64 `json:"size"`
	Free   uint64 `json:"free"`
}

const winStorageScript = `
$ErrorActionPreference='SilentlyContinue'
$disks=@(Get-PhysicalDisk -ErrorAction SilentlyContinue | ForEach-Object {
  [pscustomobject]@{ name=$_.FriendlyName; media=[string]$_.MediaType; bus=[string]$_.BusType; size=[uint64]$_.Size; health=[string]$_.HealthStatus }
})
$vols=@(Get-Volume -ErrorAction SilentlyContinue | Where-Object { $_.DriveLetter } | ForEach-Object {
  [pscustomobject]@{ letter=[string]$_.DriveLetter; fs=[string]$_.FileSystem; size=[uint64]$_.Size; free=[uint64]$_.SizeRemaining }
})
[pscustomobject]@{ disks=$disks; volumes=$vols } | ConvertTo-Json -Compress -Depth 4`

type winStorageRaw struct {
	Disks   []winDiskRaw `json:"disks"`
	Volumes []winVolRaw  `json:"volumes"`
}

func collectStorage(ctx context.Context, opts domain.DiagnosticsStorageOptions) (domain.DiagnosticsStorage, error) {
	storage := domain.DiagnosticsStorage{
		Status:   domain.DiagnosticsStatusOK,
		Disks:    []domain.DiagnosticsDisk{},
		Volumes:  []domain.DiagnosticsVolume{},
		Warnings: []string{},
	}
	if !psAvailable() {
		storage.Status = domain.DiagnosticsStatusUnsupported
		storage.Warnings = append(storage.Warnings, "powershell not found")
		return storage, nil
	}
	out, err := runPowerShell(ctx, winStorageScript)
	if err != nil {
		storage.Status = domain.DiagnosticsStatusError
		storage.Warnings = append(storage.Warnings, "could not read storage: "+firstLine(err.Error()))
		return storage, nil
	}
	var raw winStorageRaw
	if json.Unmarshal([]byte(strings.TrimSpace(out)), &raw) != nil {
		storage.Status = domain.DiagnosticsStatusPartial
		storage.Warnings = append(storage.Warnings, "could not parse storage data")
		return storage, nil
	}
	for _, d := range raw.Disks {
		storage.Disks = append(storage.Disks, domain.DiagnosticsDisk{
			Name:           strings.TrimSpace(d.Name),
			Interface:      normalizeBusType(d.Bus),
			SizeBytes:      d.Size,
			Health:         normalizeHealth(d.Health),
			TemperatureC:   nil,
			WearPercent:    nil,
			SerialRedacted: true,
		})
	}
	if opts.IncludeVolumes {
		for _, v := range raw.Volumes {
			storage.Volumes = append(storage.Volumes, domain.DiagnosticsVolume{
				Mount: v.Letter + ":", Filesystem: v.FS, SizeBytes: v.Size, FreeBytes: v.Free,
			})
		}
	}
	if opts.IncludeHealth {
		storage.Warnings = append(storage.Warnings, "SMART temperature/wear requires elevated access and is not collected")
	}
	return storage, nil
}

func normalizeBusType(bus string) string {
	switch strings.ToUpper(strings.TrimSpace(bus)) {
	case "NVME":
		return "NVMe"
	case "SATA", "ATA":
		return "SATA"
	case "USB":
		return "USB"
	case "FILE BACKED VIRTUAL", "VIRTUAL", "STORAGE SPACES":
		return "Virtual"
	case "":
		return "Unknown"
	default:
		return strings.TrimSpace(bus)
	}
}

func normalizeHealth(h string) string {
	switch strings.ToLower(strings.TrimSpace(h)) {
	case "healthy":
		return "OK"
	case "warning":
		return "Warning"
	case "unhealthy":
		return "Critical"
	default:
		return "Unknown"
	}
}

// unmarshalArray parses ConvertTo-Json output that may be a single object (one
// element) or an array. Empty output yields an empty slice.
func unmarshalArray[T any](out string) []T {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	var arr []T
	if json.Unmarshal([]byte(out), &arr) == nil {
		return arr
	}
	var one T
	if json.Unmarshal([]byte(out), &one) == nil {
		return []T{one}
	}
	return nil
}
