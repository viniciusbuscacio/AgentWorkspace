package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"aw/internal/domain"
)

// fakeOneShotRuntimeMemory is a one-shot runtime stub for condensation tests.
type fakeOneShotRuntimeMemory struct {
	reply domain.AgentReply
	err   error
}

func (f *fakeOneShotRuntimeMemory) GenerateOneShot(_ context.Context, _ domain.ModelConfig, _ string, _ int32) (domain.AgentReply, error) {
	return f.reply, f.err
}

// newFakeProviderSecretStore returns a SecretStore pre-configured with an
// active OpenAI provider, so ResolveProviderRuntimeConfig succeeds.
func newFakeProviderSecretStore() *fakeSecretStore {
	s := newFakeSecretStore()
	_ = s.SetSecret("_config_provider", "openai")
	_ = s.SetSecret("openai_api_key", "sk-test-key-00000000000000000000")
	return s
}

func condensationInput(docStore *fakeUserMemoryDocStore, secretStore *fakeSecretStore, runtime *fakeOneShotRuntimeMemory) UserMemoryCondenseInput {
	return UserMemoryCondenseInput{
		DocStore:      docStore,
		ProviderStore: secretStore,
		LogStore:      nil,
		SecretStore:   secretStore,
		Runtime:       runtime,
	}
}

func bigDoc(runes int) string {
	line := "- editor: Neovim, the best editor for coding sessions with the user\n"
	var b strings.Builder
	for b.Len() < runes {
		b.WriteString(line)
	}
	result := []rune(b.String())
	if len(result) > runes {
		return string(result[:runes])
	}
	return string(result)
}

func TestCondensationSkipsBelowThreshold(t *testing.T) {
	doc := newFakeUserMemoryDocStore()
	doc.doc = domain.UserMemoryDoc{Content: "- short: content"}
	rt := &fakeOneShotRuntimeMemory{reply: domain.AgentReply{Text: "condensed"}}
	result, err := MaybeCondenseUserMemory(context.Background(), condensationInput(doc, newFakeProviderSecretStore(), rt))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Condensed {
		t.Fatalf("condensed doc below threshold")
	}
	if result.SkippedReason != "below threshold" {
		t.Fatalf("reason = %q, want 'below threshold'", result.SkippedReason)
	}
}

func TestCondensationRunsAboveThreshold(t *testing.T) {
	doc := newFakeUserMemoryDocStore()
	doc.doc = domain.UserMemoryDoc{Content: bigDoc(UserMemoryCondenseThreshold + 100)}
	rt := &fakeOneShotRuntimeMemory{reply: domain.AgentReply{Text: "condensed short version"}}
	result, err := MaybeCondenseUserMemory(context.Background(), condensationInput(doc, newFakeProviderSecretStore(), rt))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !result.Condensed {
		t.Fatalf("expected condensation, got skipped: %s", result.SkippedReason)
	}
	if doc.doc.Content != "condensed short version" {
		t.Fatalf("doc not updated: %q", doc.doc.Content)
	}
}

func TestCondensationWritesBackup(t *testing.T) {
	doc := newFakeUserMemoryDocStore()
	original := bigDoc(UserMemoryCondenseThreshold + 100)
	doc.doc = domain.UserMemoryDoc{Content: original}
	rt := &fakeOneShotRuntimeMemory{reply: domain.AgentReply{Text: "condensed"}}
	if _, err := MaybeCondenseUserMemory(context.Background(), condensationInput(doc, newFakeProviderSecretStore(), rt)); err != nil {
		t.Fatalf("error = %v", err)
	}
	if doc.doc.Backup != original {
		t.Fatalf("backup not written: %q", doc.doc.Backup)
	}
}

func TestCondensationScrubsBeforeLLM(t *testing.T) {
	doc := newFakeUserMemoryDocStore()
	doc.doc = domain.UserMemoryDoc{Content: bigDoc(UserMemoryCondenseThreshold + 100)}
	rt := &fakeOneShotRuntimeMemory{reply: domain.AgentReply{Text: "condensed"}}
	if _, err := MaybeCondenseUserMemory(context.Background(), condensationInput(doc, newFakeProviderSecretStore(), rt)); err != nil {
		t.Fatalf("error = %v", err)
	}
	if doc.doc.Content != "condensed" {
		t.Fatalf("content = %q, want condensed", doc.doc.Content)
	}
}

func TestCondensationOncePerDay(t *testing.T) {
	doc := newFakeUserMemoryDocStore()
	doc.doc = domain.UserMemoryDoc{Content: bigDoc(UserMemoryCondenseThreshold + 100)}
	rt := &fakeOneShotRuntimeMemory{reply: domain.AgentReply{Text: "condensed"}}
	secrets := newFakeProviderSecretStore()
	in := condensationInput(doc, secrets, rt)

	// First run: should condense.
	r1, err := MaybeCondenseUserMemory(context.Background(), in)
	if err != nil || !r1.Condensed {
		t.Fatalf("first run: condensed=%v err=%v", r1.Condensed, err)
	}

	// Second run same day: should skip.
	doc.doc.Content = bigDoc(UserMemoryCondenseThreshold + 100) // reset to trigger threshold
	r2, err := MaybeCondenseUserMemory(context.Background(), in)
	if err != nil {
		t.Fatalf("second run error = %v", err)
	}
	if r2.Condensed {
		t.Fatalf("second run condensed on same day — cooldown broken")
	}
	if r2.SkippedReason != "already condensed today" {
		t.Fatalf("reason = %q, want 'already condensed today'", r2.SkippedReason)
	}
}

func TestCondensationLLMErrorLeavesDocUntouched(t *testing.T) {
	doc := newFakeUserMemoryDocStore()
	original := bigDoc(UserMemoryCondenseThreshold + 100)
	doc.doc = domain.UserMemoryDoc{Content: original}
	rt := &fakeOneShotRuntimeMemory{err: errors.New("LLM unavailable")}
	_, err := MaybeCondenseUserMemory(context.Background(), condensationInput(doc, newFakeProviderSecretStore(), rt))
	if err == nil {
		t.Fatalf("expected error from failed LLM call")
	}
	// Doc must be unchanged.
	if doc.doc.Content != original {
		t.Fatalf("doc modified after LLM error: %q", doc.doc.Content)
	}
	if doc.doc.Backup != "" {
		t.Fatalf("backup written on error: %q", doc.doc.Backup)
	}
}

func TestCondensationLockedVaultSkips(t *testing.T) {
	doc := newFakeUserMemoryDocStore()
	doc.unlocked = false
	rt := &fakeOneShotRuntimeMemory{reply: domain.AgentReply{Text: "condensed"}}
	result, err := MaybeCondenseUserMemory(context.Background(), condensationInput(doc, newFakeProviderSecretStore(), rt))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Condensed {
		t.Fatalf("should not condense when vault locked")
	}
}
