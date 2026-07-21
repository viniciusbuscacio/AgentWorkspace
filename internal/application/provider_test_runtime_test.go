package application

import (
	"context"
	"errors"
	"testing"

	"aw/internal/domain"
)

type fakeProviderTestRuntime struct {
	reply domain.AgentReply
	err   error
	gotID string
}

func (f *fakeProviderTestRuntime) GenerateOneShot(_ context.Context, cfg domain.ModelConfig, _ string, _ int32) (domain.AgentReply, error) {
	f.gotID = cfg.ProviderID
	return f.reply, f.err
}

func TestTestProviderViaRuntimeSuccess(t *testing.T) {
	rt := &fakeProviderTestRuntime{reply: domain.AgentReply{Text: "OK", Model: "claude-sonnet-4"}}
	res := TestProviderViaRuntime(context.Background(), rt, domain.ProviderRuntimeConfig{
		ProviderID: "github-copilot", AuthType: "oauth-device-code", Model: "claude-sonnet-4",
	})
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	if res.Model != "claude-sonnet-4" {
		t.Errorf("model = %q", res.Model)
	}
	if rt.gotID != "github-copilot" {
		t.Errorf("runtime got provider %q", rt.gotID)
	}
}

func TestTestProviderViaRuntimeError(t *testing.T) {
	rt := &fakeProviderTestRuntime{err: errors.New("400 Bad Request: The requested model is not supported")}
	res := TestProviderViaRuntime(context.Background(), rt, domain.ProviderRuntimeConfig{
		ProviderID: "github-copilot", AuthType: "oauth-device-code", Model: "bogus",
	})
	if res.Success || res.Error == "" {
		t.Fatalf("expected failure with error, got %+v", res)
	}
}

func TestTestProviderViaRuntimeNilRuntime(t *testing.T) {
	res := TestProviderViaRuntime(context.Background(), nil, domain.ProviderRuntimeConfig{})
	if res.Success || res.Error == "" {
		t.Fatalf("expected failure for nil runtime, got %+v", res)
	}
}
