package tools

import (
	"context"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/infrastructure/externalsafe"
)

const (
	externalDistillTimeout        = 30 * time.Second
	externalBrowserDistillTimeout = 5 * time.Second
)

type externalProcessOptions struct {
	SourceType string
	Origin     string
	Mode       domain.ExternalContentMode
	MaxChars   int
	// DistillTimeout keeps latency-sensitive surfaces, especially browser reads,
	// from blocking on an optional semantic classifier.
	DistillTimeout time.Duration
}

func (w *workspace) processExternalContent(ctx context.Context, value any, opts externalProcessOptions) domain.ExternalContentEnvelope {
	sourceType := strings.TrimSpace(opts.SourceType)
	if sourceType == "" {
		sourceType = domain.ExternalSourceUnknown
	}
	origin := strings.TrimSpace(opts.Origin)
	mode := opts.Mode
	if mode == "" {
		mode = domain.ExternalContentModeDistill
	}
	maxChars := opts.MaxChars
	if maxChars <= 0 {
		maxChars = externalsafe.DefaultMaxChars
	}

	rawText := externalContentText(value)
	started := time.Now()
	w.logExternal(ctx, ExternalLogEvent{
		Event:      "external.content.received",
		SourceType: sourceType,
		Origin:     origin,
		Mode:       mode,
		Status:     "ok",
		InputChars: len([]rune(rawText)),
	})
	pre := scanExternalContent(sourceType, origin, rawText, maxChars)
	finalScan := pre
	distilled := false
	var summary string
	var distillWarnings []string

	if w.shouldDistillExternal(pre, mode) {
		distillStarted := time.Now()
		w.logExternal(ctx, ExternalLogEvent{
			Event:      "external.content.distill_started",
			SourceType: sourceType,
			Origin:     origin,
			Mode:       mode,
			Status:     "ok",
			InputChars: pre.CharCount,
			RiskLevel:  pre.RiskLevel,
			Suspicious: pre.Suspicious,
		})
		distillCtx, cancel := context.WithTimeout(contextOrBackground(ctx), externalDistillTimeoutFor(opts))
		distill, err := w.runExternalDistiller(distillCtx, domain.ExternalDistillRequest{
			SourceType: sourceType,
			Origin:     origin,
			Mode:       mode,
			Content:    pre.BodyClean,
		})
		cancel()
		if err == nil {
			distilled = true
			summary = strings.TrimSpace(distill.Summary)
			distillWarnings = appendDistillWarnings(distillWarnings, "distiller_warning", distill.Warnings)
			distillWarnings = appendDistillWarnings(distillWarnings, "semantic_instruction_found", distill.PossibleInstructionsFound)
			finalText := strings.TrimSpace(distill.CleanText)
			if mode == domain.ExternalContentModePreserveVerbatim {
				finalText = pre.BodyClean
			}
			finalScan = scanExternalContent(sourceType, origin, finalText, maxChars)
			finalScan.Warnings = append(finalScan.Warnings, distillWarnings...)
			if len(distill.PossibleInstructionsFound) > 0 {
				finalScan.Suspicious = true
				finalScan.RiskLevel = domain.ExternalRiskHigh
			}
			w.logExternal(ctx, ExternalLogEvent{
				Event:       "external.content.distill_completed",
				SourceType:  sourceType,
				Origin:      origin,
				Mode:        mode,
				Status:      "ok",
				DurationMs:  time.Since(distillStarted).Milliseconds(),
				InputChars:  pre.CharCount,
				OutputChars: finalScan.CharCount,
				RiskLevel:   finalScan.RiskLevel,
				Suspicious:  finalScan.Suspicious,
				Distilled:   true,
				Warnings:    distillWarnings,
			})
		} else {
			w.logExternal(ctx, ExternalLogEvent{
				Event:      "external.content.distill_failed",
				SourceType: sourceType,
				Origin:     origin,
				Mode:       mode,
				Status:     "error",
				DurationMs: time.Since(distillStarted).Milliseconds(),
				InputChars: pre.CharCount,
				RiskLevel:  pre.RiskLevel,
				Suspicious: pre.Suspicious,
				Err:        err,
			})
		}
	}

	w.auditExternalDetection(ctx, sourceType, finalScan)
	envelope := externalEnvelopeForMode(value, finalScan, mode)
	envelope.Distilled = distilled
	envelope.Summary = summary
	envelope.DistillWarnings = distillWarnings
	w.logExternal(ctx, ExternalLogEvent{
		Event:       "external.content.processed",
		SourceType:  sourceType,
		Origin:      origin,
		Mode:        mode,
		Status:      "ok",
		DurationMs:  time.Since(started).Milliseconds(),
		InputChars:  len([]rune(rawText)),
		OutputChars: finalScan.CharCount,
		RiskLevel:   finalScan.RiskLevel,
		Suspicious:  finalScan.Suspicious,
		Distilled:   distilled,
		Warnings:    finalScan.Warnings,
		Attributes: map[string]any{
			"truncated": finalScan.Truncated,
		},
	})
	return envelope
}

func (w *workspace) runExternalDistiller(ctx context.Context, req domain.ExternalDistillRequest) (domain.ExternalDistillResult, error) {
	if w.externalDistillFn == nil {
		return domain.ExternalDistillResult{}, context.Canceled
	}
	type distillResult struct {
		result domain.ExternalDistillResult
		err    error
	}
	done := make(chan distillResult, 1)
	go func() {
		result, err := w.externalDistillFn(ctx, req)
		done <- distillResult{result: result, err: err}
	}()
	select {
	case out := <-done:
		return out.result, out.err
	case <-ctx.Done():
		return domain.ExternalDistillResult{}, ctx.Err()
	}
}

func (w *workspace) logExternal(ctx context.Context, event ExternalLogEvent) {
	if w.logExternalFn == nil {
		return
	}
	w.logExternalFn(ctx, event)
}

func (w *workspace) shouldDistillExternal(scan externalsafe.Result, mode domain.ExternalContentMode) bool {
	if w.externalDistillFn == nil {
		return false
	}
	if strings.TrimSpace(scan.BodyClean) == "" {
		return false
	}
	if mode == domain.ExternalContentModePreserveVerbatim && !scan.Suspicious && scan.RiskLevel != domain.ExternalRiskHigh {
		return false
	}
	return scan.Suspicious || scan.RiskLevel == domain.ExternalRiskHigh
}

func externalDistillTimeoutFor(opts externalProcessOptions) time.Duration {
	if opts.DistillTimeout > 0 {
		return opts.DistillTimeout
	}
	switch strings.TrimSpace(opts.Origin) {
	case "browser.tabs", "browser.snapshot", "browser.cdp":
		return externalBrowserDistillTimeout
	default:
		return externalDistillTimeout
	}
}

func externalEnvelopeForMode(value any, scan externalsafe.Result, mode domain.ExternalContentMode) domain.ExternalContentEnvelope {
	if text, ok := value.(string); ok {
		content := any(scan.BodyClean)
		rawReplaced := strings.TrimSpace(text) != strings.TrimSpace(scan.BodyClean)
		if mode == domain.ExternalContentModePreserveVerbatim {
			content = text
			rawReplaced = false
		}
		return domain.ExternalContentEnvelope{
			SafeContent:    scan.BodyClean,
			Content:        content,
			ExternalSafety: safetyMetadata(scan),
			CharCount:      scan.CharCount,
			Truncated:      scan.Truncated,
			RawReplaced:    rawReplaced,
		}
	}
	if mode == domain.ExternalContentModeDistill {
		return domain.ExternalContentEnvelope{
			SafeContent:    scan.BodyClean,
			Content:        scan.BodyClean,
			ExternalSafety: safetyMetadata(scan),
			CharCount:      scan.CharCount,
			Truncated:      scan.Truncated,
			RawReplaced:    true,
		}
	}
	return domain.ExternalContentEnvelope{
		SafeContent:    scan.BodyClean,
		Content:        scan.BodyClean,
		RawContent:     value,
		ExternalSafety: safetyMetadata(scan),
		CharCount:      scan.CharCount,
		Truncated:      scan.Truncated,
		RawReplaced:    true,
	}
}

func appendDistillWarnings(out []string, prefix string, values []string) []string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, prefix+": "+value)
		if len(out) >= 24 {
			break
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}
