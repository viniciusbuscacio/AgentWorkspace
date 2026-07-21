package tools

import (
	"context"
	"fmt"
	"strings"

	"aw/internal/domain"
)

// DiagnosticsFuncs wires the native system-diagnostics use cases into the aw
// action gateway. Each callback delegates to the application diagnostics service
// (defaults, timeouts, caps, redaction, policy gate) which calls the OS probe.
// The current Permissions mode is read per call from the workspace and passed
// through, so block_all gating and detailed inspection both come from one source.
type DiagnosticsFuncs struct {
	Capabilities func(ctx context.Context, mode domain.SandboxMode) (any, error)
	Summary      func(ctx context.Context, mode domain.SandboxMode, opts domain.DiagnosticsSummaryOptions) (any, error)
	Report       func(ctx context.Context, mode domain.SandboxMode, opts domain.DiagnosticsReportOptions) (any, error)
	Sensors      func(ctx context.Context, mode domain.SandboxMode, opts domain.DiagnosticsSensorsOptions) (any, error)
	Storage      func(ctx context.Context, mode domain.SandboxMode, opts domain.DiagnosticsStorageOptions) (any, error)
	Processes    func(ctx context.Context, mode domain.SandboxMode, opts domain.DiagnosticsProcessesOptions) (any, error)
	Devices      func(ctx context.Context, mode domain.SandboxMode, opts domain.DiagnosticsDevicesOptions) (any, error)
	Logs         func(ctx context.Context, mode domain.SandboxMode, opts domain.DiagnosticsLogsOptions) (any, error)
	LogsSummary  func(ctx context.Context, mode domain.SandboxMode, opts domain.DiagnosticsLogsSummaryOptions) (any, error)
}

// registerDiagnosticsActions wires the diagnostics.* actions. Diagnostics is a
// product capability, not a self-dev one, so it registers whenever the probe is
// wired — independent of SelfManage. diagnostics.capabilities works in every
// mode (including block_all); detailed actions are gated inside the application.
func registerDiagnosticsActions(reg map[string]AwActionHandler) {
	reg["diagnostics.capabilities"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		if w.diagnostics == nil || w.diagnostics.Capabilities == nil {
			return "", errUnavailable("diagnostics")
		}
		result, err := w.diagnostics.Capabilities(ctx, w.sandboxMode())
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["diagnostics.summary"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.diagnostics == nil || w.diagnostics.Summary == nil {
			return "", errUnavailable("diagnostics")
		}
		includeRuntime, err := awBoolArg(args, "includeRuntime", true)
		if err != nil {
			return "", err
		}
		result, err := w.diagnostics.Summary(ctx, w.sandboxMode(), domain.DiagnosticsSummaryOptions{IncludeRuntime: includeRuntime})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["diagnostics.report"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.diagnostics == nil || w.diagnostics.Report == nil {
			return "", errUnavailable("diagnostics")
		}
		sections, err := awStringSliceArg(args, "sections")
		if err != nil {
			return "", err
		}
		timeoutMs, _, err := awIntArg(args, "timeoutMs")
		if err != nil {
			return "", err
		}
		redaction, _, err := awStringArg(args, "redaction")
		if err != nil {
			return "", err
		}
		result, err := w.diagnostics.Report(ctx, w.sandboxMode(), domain.DiagnosticsReportOptions{
			Sections: sections, TimeoutMs: timeoutMs, Redaction: redaction,
		})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["diagnostics.sensors"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.diagnostics == nil || w.diagnostics.Sensors == nil {
			return "", errUnavailable("diagnostics")
		}
		timeoutMs, _, err := awIntArg(args, "timeoutMs")
		if err != nil {
			return "", err
		}
		result, err := w.diagnostics.Sensors(ctx, w.sandboxMode(), domain.DiagnosticsSensorsOptions{TimeoutMs: timeoutMs})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["diagnostics.storage"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.diagnostics == nil || w.diagnostics.Storage == nil {
			return "", errUnavailable("diagnostics")
		}
		includeHealth, err := awBoolArg(args, "includeHealth", true)
		if err != nil {
			return "", err
		}
		includeVolumes, err := awBoolArg(args, "includeVolumes", true)
		if err != nil {
			return "", err
		}
		result, err := w.diagnostics.Storage(ctx, w.sandboxMode(), domain.DiagnosticsStorageOptions{
			IncludeHealth: includeHealth, IncludeVolumes: includeVolumes,
		})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["diagnostics.processes"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.diagnostics == nil || w.diagnostics.Processes == nil {
			return "", errUnavailable("diagnostics")
		}
		sortBy, _, err := awStringArg(args, "sortBy")
		if err != nil {
			return "", err
		}
		limit, _, err := awIntArg(args, "limit")
		if err != nil {
			return "", err
		}
		sampleMs, _, err := awIntArg(args, "sampleMs")
		if err != nil {
			return "", err
		}
		result, err := w.diagnostics.Processes(ctx, w.sandboxMode(), domain.DiagnosticsProcessesOptions{
			SortBy: sortBy, Limit: limit, SampleMs: sampleMs,
		})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["diagnostics.devices"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.diagnostics == nil || w.diagnostics.Devices == nil {
			return "", errUnavailable("diagnostics")
		}
		mode, _, err := awStringArg(args, "mode")
		if err != nil {
			return "", err
		}
		includeDrivers, err := awBoolArg(args, "includeDrivers", true)
		if err != nil {
			return "", err
		}
		includeDisabled, err := awBoolArg(args, "includeDisabled", true)
		if err != nil {
			return "", err
		}
		includeProblem, err := awBoolArg(args, "includeProblemDevices", true)
		if err != nil {
			return "", err
		}
		classes, err := awStringSliceArg(args, "classes")
		if err != nil {
			return "", err
		}
		limit, _, err := awIntArg(args, "limit")
		if err != nil {
			return "", err
		}
		redaction, _, err := awStringArg(args, "redaction")
		if err != nil {
			return "", err
		}
		result, err := w.diagnostics.Devices(ctx, w.sandboxMode(), domain.DiagnosticsDevicesOptions{
			Mode: mode, IncludeDrivers: includeDrivers, IncludeDisabled: includeDisabled,
			IncludeProblemDevice: includeProblem, Classes: classes, Limit: limit, Redaction: redaction,
		})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["diagnostics.logs"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.diagnostics == nil || w.diagnostics.Logs == nil {
			return "", errUnavailable("diagnostics")
		}
		opts, err := parseLogsOptions(args)
		if err != nil {
			return "", err
		}
		result, err := w.diagnostics.Logs(ctx, w.sandboxMode(), opts)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["diagnostics.logs.summary"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.diagnostics == nil || w.diagnostics.LogsSummary == nil {
			return "", errUnavailable("diagnostics")
		}
		since, _, err := awStringArg(args, "since")
		if err != nil {
			return "", err
		}
		focus, _, err := awStringArg(args, "focus")
		if err != nil {
			return "", err
		}
		result, err := w.diagnostics.LogsSummary(ctx, w.sandboxMode(), domain.DiagnosticsLogsSummaryOptions{
			Since: since, Focus: focus,
		})
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}

func parseLogsOptions(args map[string]any) (domain.DiagnosticsLogsOptions, error) {
	since, _, err := awStringArg(args, "since")
	if err != nil {
		return domain.DiagnosticsLogsOptions{}, err
	}
	until, _, err := awStringArg(args, "until")
	if err != nil {
		return domain.DiagnosticsLogsOptions{}, err
	}
	severity, err := awStringSliceArg(args, "severity")
	if err != nil {
		return domain.DiagnosticsLogsOptions{}, err
	}
	sources, err := awStringSliceArg(args, "sources")
	if err != nil {
		return domain.DiagnosticsLogsOptions{}, err
	}
	query, _, err := awStringArg(args, "query")
	if err != nil {
		return domain.DiagnosticsLogsOptions{}, err
	}
	limit, _, err := awIntArg(args, "limit")
	if err != nil {
		return domain.DiagnosticsLogsOptions{}, err
	}
	redaction, _, err := awStringArg(args, "redaction")
	if err != nil {
		return domain.DiagnosticsLogsOptions{}, err
	}
	return domain.DiagnosticsLogsOptions{
		Since: since, Until: until, Severity: severity, Sources: sources,
		Query: query, Limit: limit, Redaction: redaction,
	}, nil
}

// awStringSliceArg parses a string-array argument. It accepts a JSON array of
// strings or a single comma-separated string; absent ⇒ nil.
func awStringSliceArg(args map[string]any, name string) ([]string, error) {
	value, ok := args[name]
	if !ok || value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case string:
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s must be an array of strings", name)
			}
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings", name)
	}
}
