package application

import "testing"

func TestResolveLocalVoiceModelDefaultsToTiny(t *testing.T) {
	store := newMemoryProviderStore()
	if got := ResolveLocalVoiceModel(store); got != "tiny" {
		t.Fatalf("ResolveLocalVoiceModel() = %q, want tiny", got)
	}
}

func TestResolveLocalVoiceModelReadsVaultSecret(t *testing.T) {
	store := newMemoryProviderStore()
	store.values[voiceTranscriptionModelSecret] = "small"
	if got := ResolveLocalVoiceModel(store); got != "small" {
		t.Fatalf("ResolveLocalVoiceModel() = %q, want small", got)
	}
}
