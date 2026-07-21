package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aw/internal/domain"
)

type winDeviceRaw struct {
	Name     string `json:"name"`
	Class    string `json:"class"`
	Mfr      string `json:"mfr"`
	Code     int    `json:"code"`
	Provider string `json:"provider"`
	Version  string `json:"version"`
	Signed   *bool  `json:"signed"`
}

type winDevicesRaw struct {
	Total    int            `json:"total"`
	Disabled int            `json:"disabled"`
	Missing  int            `json:"missing"`
	ByClass  map[string]int `json:"byClass"`
	Rows     []winDeviceRaw `json:"rows"`
}

func collectDevices(ctx context.Context, opts domain.DiagnosticsDevicesOptions) (domain.DiagnosticsDevices, error) {
	result := domain.DiagnosticsDevices{
		Status:   domain.DiagnosticsStatusOK,
		Mode:     opts.Mode,
		Devices:  []domain.DiagnosticsDevice{},
		Summary:  domain.DiagnosticsDevicesSummary{ByClass: map[string]int{}},
		Warnings: []string{},
		CorrelationHints: []domain.DiagnosticsCorrelationHint{
			{Provider: "Kernel-PnP", Reason: "check recent device setup/failure events when a device shows a problem status"},
			{Provider: "DeviceSetupManager", Reason: "driver install/update history for problem devices"},
		},
	}
	if !psAvailable() {
		result.Status = domain.DiagnosticsStatusUnsupported
		result.Warnings = append(result.Warnings, "powershell not found")
		return result, nil
	}
	// rowSource selects which devices become rows; summary always reflects the
	// full inventory. "summary" mode emits no rows at all.
	rowSource := "@($all | Where-Object { $_.ConfigManagerErrorCode -ne 0 })"
	switch opts.Mode {
	case "all":
		rowSource = fmt.Sprintf("@($all | Select-Object -First %d)", opts.Limit)
	case "summary":
		rowSource = "@()"
	}
	script := fmt.Sprintf(`
$ErrorActionPreference='SilentlyContinue'
$all=@(Get-CimInstance Win32_PnPEntity)
$drivers=@{}
foreach($d in Get-CimInstance Win32_PnPSignedDriver){ if($d.DeviceID){ $drivers[$d.DeviceID]=$d } }
$disabled=@($all | Where-Object { $_.ConfigManagerErrorCode -eq 22 }).Count
$missing=@($all | Where-Object { $_.ConfigManagerErrorCode -eq 28 }).Count
$byClass=@{}
foreach($e in $all){ $k=[string]$e.PNPClass; if(!$k){$k='Unknown'}; if($byClass.ContainsKey($k)){$byClass[$k]++}else{$byClass[$k]=1} }
$src=%s
$rows=@(foreach($e in $src){
  $dv=$drivers[$e.PNPDeviceID]
  [pscustomobject]@{ name=$e.Name; class=[string]$e.PNPClass; mfr=$e.Manufacturer; code=[int]$e.ConfigManagerErrorCode; provider=$dv.DriverProviderName; version=$dv.DriverVersion; signed=$dv.IsSigned }
})
[pscustomobject]@{ total=$all.Count; disabled=$disabled; missing=$missing; byClass=$byClass; rows=$rows } | ConvertTo-Json -Compress -Depth 4`, rowSource)

	out, err := runPowerShell(ctx, script)
	if err != nil {
		result.Status = domain.DiagnosticsStatusError
		result.Warnings = append(result.Warnings, "could not read devices: "+firstLine(err.Error()))
		return result, nil
	}
	var raw winDevicesRaw
	if json.Unmarshal([]byte(strings.TrimSpace(out)), &raw) != nil {
		result.Status = domain.DiagnosticsStatusPartial
		result.Warnings = append(result.Warnings, "could not parse device data")
		return result, nil
	}

	classWanted := map[string]bool{}
	for _, c := range opts.Classes {
		classWanted[c] = true
	}
	problemCount := 0
	for _, r := range raw.Rows {
		class := normalizeDeviceClass(r.Class)
		status := winDeviceStatus(r.Code)
		if status != "ok" {
			problemCount++
		}
		if len(classWanted) > 0 && !classWanted[class] {
			continue
		}
		if status == "disabled" && !opts.IncludeDisabled && opts.Mode == "problems" {
			continue
		}
		dev := domain.DiagnosticsDevice{
			Name:             redactDeviceName(r.Name),
			Class:            class,
			Status:           status,
			Manufacturer:     strings.TrimSpace(r.Mfr),
			IDsRedacted:      true,
			LocationRedacted: true,
		}
		if opts.IncludeDrivers && (r.Provider != "" || r.Version != "" || r.Signed != nil) {
			dev.Driver = &domain.DiagnosticsDriver{
				Provider:     strings.TrimSpace(r.Provider),
				Version:      strings.TrimSpace(r.Version),
				Signed:       r.Signed,
				PathRedacted: true,
			}
		}
		result.Devices = append(result.Devices, dev)
		if len(result.Devices) >= opts.Limit {
			result.Warnings = append(result.Warnings, "device list truncated to the limit")
			break
		}
	}

	result.Summary = domain.DiagnosticsDevicesSummary{
		Total:          raw.Total,
		ProblemDevices: problemCount,
		Disabled:       raw.Disabled,
		MissingDrivers: raw.Missing,
		ByClass:        normalizeClassCounts(raw.ByClass),
	}
	if problemCount == 0 && opts.Mode == "problems" {
		result.Warnings = append(result.Warnings, "no problem devices found; current inventory looks healthy. Recent failures still require diagnostics.logs correlation.")
	}
	return result, nil
}

func winDeviceStatus(code int) string {
	switch code {
	case 0:
		return "ok"
	case 22:
		return "disabled"
	case 28, 18, 31:
		return "missing_driver"
	case 37, 39, 41, 52:
		return "driver_error"
	default:
		return "device_error"
	}
}

func normalizeDeviceClass(class string) string {
	switch strings.ToLower(strings.TrimSpace(class)) {
	case "display", "monitor":
		return "display"
	case "diskdrive", "volume", "scsiadapter", "hdc", "storagevolume":
		return "storage"
	case "net", "netadapter":
		return "network"
	case "battery":
		return "battery"
	case "usb", "usbdevice":
		return "usb"
	case "bluetooth":
		return "bluetooth"
	case "media", "audioendpoint", "sound", "audioprocessingobject":
		return "audio"
	case "camera", "image":
		return "camera"
	case "keyboard", "mouse", "hidclass", "hid":
		return "input"
	case "system", "computer", "processor", "firmware":
		return "system"
	case "":
		return "unknown"
	default:
		return "unknown"
	}
}

func normalizeClassCounts(raw map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range raw {
		out[normalizeDeviceClass(k)] += v
	}
	return out
}
