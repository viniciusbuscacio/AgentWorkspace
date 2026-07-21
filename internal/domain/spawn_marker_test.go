package domain

import "testing"

func TestSpawnMarkerShape(t *testing.T) {
	if got := SpawnMarker("run-1"); got != `::spawn{run="run-1"}` {
		t.Fatalf("SpawnMarker() = %q", got)
	}
}

func TestStripSpawnMarkers(t *testing.T) {
	text := "Vou disparar os agentes:\n\n" + SpawnMarker("run-1") + "\n\nPronto, o resultado foi X."
	if got := StripSpawnMarkers(text); got != "Vou disparar os agentes:\n\nPronto, o resultado foi X." {
		t.Fatalf("StripSpawnMarkers() = %q", got)
	}
}

func TestStripSpawnMarkersLeavesPlainTextAlone(t *testing.T) {
	text := "no markers here\n\njust prose"
	if got := StripSpawnMarkers(text); got != text {
		t.Fatalf("StripSpawnMarkers() = %q, want unchanged", got)
	}
}

func TestStripSpawnMarkersRemovesResultsBlocks(t *testing.T) {
	block, err := SpawnResultsBlock("run-1", []SpawnCardTask{{ID: "t1", Task: "subagent 1", Status: "success", Output: "ok", ElapsedMs: 10}})
	if err != nil {
		t.Fatalf("SpawnResultsBlock() error = %v", err)
	}
	text := "Antes.\n\n" + block + "\n\nDepois."
	if got := StripSpawnMarkers(text); got != "Antes.\n\nDepois." {
		t.Fatalf("StripSpawnMarkers() = %q", got)
	}
}

func TestSpawnResultsBlockShape(t *testing.T) {
	block, err := SpawnResultsBlock("run-1", []SpawnCardTask{{ID: "t1", Task: "subagent 1", Status: "success", Output: "ok", ElapsedMs: 10}})
	if err != nil {
		t.Fatalf("SpawnResultsBlock() error = %v", err)
	}
	want := "::spawn{run=\"run-1\"}\n{\"tasks\":[{\"id\":\"t1\",\"task\":\"subagent 1\",\"status\":\"success\",\"output\":\"ok\",\"elapsedMs\":10}]}\n::end-spawn"
	if block != want {
		t.Fatalf("SpawnResultsBlock() = %q, want %q", block, want)
	}
}

func TestReplaceSpawnMarkers(t *testing.T) {
	text := "a\n" + SpawnMarker("run-1") + "\nb\n" + SpawnMarker("run-2") + "\nc"
	got := ReplaceSpawnMarkers(text, func(runID string) (string, bool) {
		if runID == "run-1" {
			return "SUBSTITUIDO", true
		}
		return "", false
	})
	if got != "a\nSUBSTITUIDO\nb\n"+SpawnMarker("run-2")+"\nc" {
		t.Fatalf("ReplaceSpawnMarkers() = %q", got)
	}
}
