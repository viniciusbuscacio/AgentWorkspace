package application

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeOneShotRuntime struct {
	reply  string
	err    error
	prompt string
}

func (f *fakeOneShotRuntime) GenerateOneShot(_ context.Context, _ domain.ModelConfig, text string, _ int32) (domain.AgentReply, error) {
	f.prompt = text
	return domain.AgentReply{Text: f.reply}, f.err
}

func TestCleanVoiceTranscriptCorrectsText(t *testing.T) {
	runtime := &fakeOneShotRuntime{reply: "\"Olá, mundo!\"\n"}
	text, corrected := CleanVoiceTranscript(context.Background(), runtime, domain.ModelConfig{}, "ola mundo", "")
	if !corrected || text != "Olá, mundo!" {
		t.Fatalf("CleanVoiceTranscript = %q corrected=%v", text, corrected)
	}
	if !strings.Contains(runtime.prompt, "ola mundo") {
		t.Fatalf("prompt did not include raw text: %q", runtime.prompt)
	}
}

func TestCleanVoiceTranscriptFallsBackToRaw(t *testing.T) {
	if text, corrected := CleanVoiceTranscript(context.Background(), nil, domain.ModelConfig{}, "bruto", ""); corrected || text != "bruto" {
		t.Fatalf("nil runtime: %q %v", text, corrected)
	}
	failing := &fakeOneShotRuntime{err: fmt.Errorf("provider offline")}
	if text, corrected := CleanVoiceTranscript(context.Background(), failing, domain.ModelConfig{}, "bruto", ""); corrected || text != "bruto" {
		t.Fatalf("error: %q %v", text, corrected)
	}
	empty := &fakeOneShotRuntime{reply: "   "}
	if text, corrected := CleanVoiceTranscript(context.Background(), empty, domain.ModelConfig{}, "bruto", ""); corrected || text != "bruto" {
		t.Fatalf("empty reply: %q %v", text, corrected)
	}
	same := &fakeOneShotRuntime{reply: "bruto"}
	if text, corrected := CleanVoiceTranscript(context.Background(), same, domain.ModelConfig{}, "bruto", ""); corrected || text != "bruto" {
		t.Fatalf("unchanged reply: %q %v", text, corrected)
	}
}

func TestPreferredLanguageFromMemory(t *testing.T) {
	if got := PreferredLanguageFromMemory("- editor: Neovim\n- preferred-language: Português\n"); got != "Português" {
		t.Fatalf("got %q, want Português", got)
	}
	if got := PreferredLanguageFromMemory("Preferred Language = Spanish."); got != "Spanish" {
		t.Fatalf("got %q, want Spanish", got)
	}
	if got := PreferredLanguageFromMemory("- editor: Neovim"); got != DefaultVoiceLanguage {
		t.Fatalf("got %q, want default", got)
	}
}

func TestVoiceCleanupPromptCarriesLanguageHint(t *testing.T) {
	prompt := VoiceCleanupPrompt("ola", "Português")
	if !strings.Contains(prompt, "preferred language is Português") {
		t.Fatalf("prompt missing language hint: %s", prompt)
	}
	if !strings.Contains(VoiceCleanupPrompt("hi", ""), "preferred language is "+DefaultVoiceLanguage) {
		t.Fatalf("empty language must default to %s", DefaultVoiceLanguage)
	}
}
