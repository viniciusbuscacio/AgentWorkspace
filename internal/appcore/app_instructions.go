package appcore

import (
	"context"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/tools"
	"aw/skills"
)

// Agent Instructions panel bindings (Settings → Agent Instructions). Reads/
// writes go through application use cases over the vault app-document store;
// every mutation refreshes the agent's injected instruction context so changes
// take effect live (no restart). The effective block uses the SAME composer as
// the runtime, only scrubbed for display.

func (a *App) ListInstructionDocuments() dto.InstructionsListResult {
	docs, err := application.ListInstructionDocuments(a.vault, a.skillsPromptInput())
	if err != nil {
		return dto.InstructionsListResult{Success: false, Error: err.Error(), Documents: []domain.InstructionDocument{}}
	}
	return dto.InstructionsListResult{Success: true, Documents: docs}
}

func (a *App) GetInstructionDocument(id string) dto.InstructionReadResult {
	doc, err := application.GetInstructionDocument(a.vault, a.skillsPromptInput(), id)
	if err != nil {
		return dto.InstructionReadResult{Success: false, Error: err.Error()}
	}
	return dto.InstructionReadResult{Success: true, Document: &doc}
}

func (a *App) SaveInstructionDocument(id string, content string) dto.InstructionSaveResult {
	if err := application.SaveInstructionDocument(a.vault, id, content); err != nil {
		return dto.InstructionSaveResult{Success: false, Error: err.Error()}
	}
	return a.instructionMutationResponse(id)
}

func (a *App) ResetInstructionDocument(id string) dto.InstructionSaveResult {
	seed, hasSeed, _ := skills.BundledAgentsDoc()
	if err := application.ResetInstructionDocument(a.vault, seed, hasSeed, id); err != nil {
		return dto.InstructionSaveResult{Success: false, Error: err.Error()}
	}
	return a.instructionMutationResponse(id)
}

func (a *App) GetEffectiveInstructions(includeContent bool) dto.EffectiveInstructionsResult {
	eff := application.ComposeEffectiveInstructions(a.vault, a.skillsPromptInput(), domain.EffectiveInstructionsScrubbed)
	if !includeContent {
		eff.Content = ""
	}
	eff.Metadata.GeneratedAt = a.now()
	return dto.EffectiveInstructionsResult{Success: true, Effective: &eff, GeneratedAt: eff.Metadata.GeneratedAt}
}

func (a *App) GetInstructionSources() dto.InstructionSourcesResult {
	sources := application.InstructionSourcesInventory(a.vault, a.skillsPromptInput())
	return dto.InstructionSourcesResult{Success: true, Sources: sources}
}

// instructionMutationResponse refreshes the runtime instruction context (unless
// a dev override masks the effective AGENTS.md) and reports it honestly, then
// attaches the updated document view.
func (a *App) instructionMutationResponse(id string) dto.InstructionSaveResult {
	input := a.skillsPromptInput()
	resp := dto.InstructionSaveResult{Success: true}
	// USER.md is always vault-backed, so a dev override never masks it. AGENTS.md
	// IS masked while AW_SKILLS_DIR serves the effective runtime AGENTS.md.
	masked := input.Active && id == domain.AgentsDocumentID
	if masked {
		resp.DevOverrideActive = true
		resp.ContextRefreshed = false
		resp.EffectiveChanged = false
		resp.Warning = "AW_SKILLS_DIR is active; the change was saved to the vault but the live AGENTS.md is currently served from the dev override."
	} else {
		application.RefreshSkillsContext(a.agent, a.vault, input)
		resp.DevOverrideActive = input.Active
		resp.ContextRefreshed = true
		resp.EffectiveChanged = true
	}
	if doc, err := application.GetInstructionDocument(a.vault, input, id); err == nil {
		view := domain.InstructionDocument{ID: doc.ID, Origin: doc.Origin, Status: doc.Status, Editable: doc.Editable, Resettable: doc.Resettable, Enabled: true}
		resp.Document = &view
	}
	return resp
}

// now returns the current UTC RFC3339 timestamp for instruction metadata.
func (a *App) now() string { return time.Now().UTC().Format(time.RFC3339) }

// instructionFuncs builds the instructions.* aw action callbacks (REST/MCP and
// the in-app agent). They reuse the same Wails-binding helpers so the agent and
// the UI cannot drift, and they return the spec response shapes.
func (a *App) instructionFuncs() *tools.InstructionFuncs {
	return &tools.InstructionFuncs{
		List: func(_ context.Context) (any, error) {
			docs, err := application.ListInstructionDocuments(a.vault, a.skillsPromptInput())
			if err != nil {
				return nil, err
			}
			return map[string]any{"documents": docs}, nil
		},
		Read: func(_ context.Context, id string) (any, error) {
			doc, err := application.GetInstructionDocument(a.vault, a.skillsPromptInput(), id)
			if err != nil {
				return nil, err
			}
			return map[string]any{"document": doc}, nil
		},
		Save: func(_ context.Context, id, content string, _ bool) (any, error) {
			if err := application.SaveInstructionDocument(a.vault, id, content); err != nil {
				return nil, err
			}
			return a.instructionMutationResponse(id), nil
		},
		Reset: func(_ context.Context, id string) (any, error) {
			seed, hasSeed, _ := skills.BundledAgentsDoc()
			if err := application.ResetInstructionDocument(a.vault, seed, hasSeed, id); err != nil {
				return nil, err
			}
			return a.instructionMutationResponse(id), nil
		},
		Effective: func(_ context.Context, includeContent bool) (any, error) {
			eff := application.ComposeEffectiveInstructions(a.vault, a.skillsPromptInput(), domain.EffectiveInstructionsScrubbed)
			out := map[string]any{
				"sources":           eff.Sources,
				"redacted":          eff.Redacted,
				"charCount":         eff.Metadata.CharCount,
				"devOverrideActive": eff.Metadata.DevOverrideActive,
				"generatedAt":       a.now(),
			}
			if includeContent {
				out["content"] = eff.Content
			}
			return out, nil
		},
		Sources: func(_ context.Context) (any, error) {
			return map[string]any{"sources": application.InstructionSourcesInventory(a.vault, a.skillsPromptInput())}, nil
		},
	}
}
