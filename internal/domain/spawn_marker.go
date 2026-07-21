package domain

import (
	"encoding/json"
	"regexp"
	"strings"
)

// The chat UI renders the live system.spawn card inline: the runtime injects a
// marker line into the assistant reply at the exact point the spawn call
// happened, and the frontend replaces that line with the card (text before the
// call -> card -> text after). When the reply is persisted, the bare marker is
// upgraded to a results block so the card survives reload and app restart:
//
//	::spawn{run="<runId>"}
//	{"tasks":[{"id","task","status","output","elapsedMs"}]}
//	::end-spawn
//
// The marker shares the `::name{attrs}` shape of the ::subagent blocks the
// frontend already parses.

var (
	spawnMarkerLine    = regexp.MustCompile(`(?m)^::spawn\{run="[^"\n]*"\}[ \t]*$`)
	spawnMarkerCapture = regexp.MustCompile(`(?m)^::spawn\{run="([^"\n]*)"\}[ \t]*$`)
	spawnResultsBlock  = regexp.MustCompile(`(?ms)^::spawn\{run="[^"\n]*"\}[ \t]*$.*?^::end-spawn[ \t]*$`)
)

// SpawnCardTask is one task row of a persisted spawn card, JSON-shaped exactly
// like the frontend's SubagentTask so the card renders from content alone.
type SpawnCardTask struct {
	ID        string `json:"id"`
	Task      string `json:"task"`
	Status    string `json:"status"`
	Output    string `json:"output,omitempty"`
	ElapsedMs int64  `json:"elapsedMs,omitempty"`
}

// SpawnMarker is the inline marker for one run's spawn card. One line, keyed by
// the chat runId the chat:subagent events carry, so the frontend can pair the
// marker with the live card state for that run.
func SpawnMarker(runID string) string {
	return `::spawn{run="` + runID + `"}`
}

// SpawnResultsBlock is the persisted form of the marker: the marker line plus
// the tasks payload, closed by ::end-spawn.
func SpawnResultsBlock(runID string, tasks []SpawnCardTask) (string, error) {
	payload, err := json.Marshal(map[string][]SpawnCardTask{"tasks": tasks})
	if err != nil {
		return "", err
	}
	return SpawnMarker(runID) + "\n" + string(payload) + "\n::end-spawn", nil
}

// ReplaceSpawnMarkers rewrites each bare marker line via build(runID); build
// returns false to leave that marker untouched (no data for the run).
func ReplaceSpawnMarkers(text string, build func(runID string) (string, bool)) string {
	if !strings.Contains(text, "::spawn{") {
		return text
	}
	return spawnMarkerCapture.ReplaceAllStringFunc(text, func(line string) string {
		match := spawnMarkerCapture.FindStringSubmatch(line)
		if len(match) != 2 {
			return line
		}
		if block, ok := build(match[1]); ok {
			return block
		}
		return line
	})
}

// StripSpawnMarkers removes spawn markers and results blocks from assistant
// text. Used when persisted messages are re-seeded into model history
// (compaction), so the model never sees UI markers as if they were its own
// words.
func StripSpawnMarkers(text string) string {
	if !strings.Contains(text, "::spawn{") {
		return text
	}
	cleaned := spawnResultsBlock.ReplaceAllString(text, "")
	cleaned = spawnMarkerLine.ReplaceAllString(cleaned, "")
	// Collapse the blank lines the removed markers leave behind.
	for strings.Contains(cleaned, "\n\n\n") {
		cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(cleaned)
}
