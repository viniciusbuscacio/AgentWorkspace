package application

import (
	"context"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// providerTestPrompt is a minimal one-shot used to confirm a provider is
// reachable end to end. Kept tiny to minimize latency and token spend.
const providerTestPrompt = "Reply with the single word: OK"
const providerTestMaxOutputTokens = 32

// TestProviderViaRuntime exercises a provider by asking its real runtime model
// for a one-shot reply. Unlike the direct HTTP probe (which only supports
// API-key providers), this runs the full adapter path — OAuth token exchange,
// derived base URLs, editor headers and model enablement — so OpenAI
// Subscription and GitHub Copilot can be validated too. No token material is
// ever placed in the result.
func TestProviderViaRuntime(ctx context.Context, runtime ports.ChatTitleRuntime, cfg domain.ProviderRuntimeConfig) domain.ProviderTestResult {
	if runtime == nil {
		return domain.ProviderTestResult{Success: false, Error: "agent runtime is not available"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	reply, err := runtime.GenerateOneShot(ctx, ModelConfigFromProviderRuntimeConfig(cfg), providerTestPrompt, providerTestMaxOutputTokens)
	latency := time.Since(start)
	if err != nil {
		return domain.ProviderTestResult{Success: false, Error: err.Error()}
	}
	model := strings.TrimSpace(reply.Model)
	if model == "" {
		model = cfg.Model
	}
	return domain.ProviderTestResult{
		Success: true,
		Latency: latency.Round(time.Millisecond).String(),
		Model:   model,
	}
}
