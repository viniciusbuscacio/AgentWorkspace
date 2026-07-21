// Base system instruction loading: the canonical text lives in
// prompts/base.md (embedded at build time) so it can be edited as prose, with
// an optional disk override for live iteration without recompiling.
package agent

import (
	_ "embed"
	"os"
	"strings"
)

//go:embed prompts/base.md
var embeddedBaseInstruction string

// baseInstructionFileEnv points to an alternative base-prompt file on disk.
// When set and readable, it replaces the embedded prompts/base.md — intended
// for prompt iteration during development only.
const baseInstructionFileEnv = "aw_BASE_PROMPT_FILE"

func baseInstruction() string {
	if path := strings.TrimSpace(os.Getenv(baseInstructionFileEnv)); path != "" {
		if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) != "" {
			return strings.TrimSpace(string(data))
		}
	}
	return strings.TrimSpace(embeddedBaseInstruction)
}
