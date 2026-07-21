package appcore

import (
	"strings"
	"testing"

	"aw/internal/domain"
	"aw/internal/infrastructure/subagent"
)

func TestSpawnRecorderDecoratesMarkerWithResults(t *testing.T) {
	recorder := newSpawnRecorder()
	recorder.observe("run-1", subagent.SpawnStatus{
		Phase: subagent.SpawnPhaseStart,
		Tasks: []subagent.SpawnTaskRef{{ID: "t1", Task: "subagent 1"}, {ID: "t2", Task: "subagent 2"}},
	})
	recorder.observe("run-1", subagent.SpawnStatus{
		Phase: subagent.SpawnPhaseEnd,
		Results: []subagent.SpawnResult{
			{ID: "t1", Status: "success", Output: "subagent 1 ok", ElapsedMs: 1200},
			{ID: "t2", Status: "timeout", Output: "", ElapsedMs: 30000},
		},
	})

	text := "antes\n\n" + domain.SpawnMarker("run-1") + "\n\ndepois"
	decorated := recorder.decorate(text)
	for _, want := range []string{`::spawn{run="run-1"}`, `"task":"subagent 1"`, `"status":"success"`, `"output":"subagent 1 ok"`, `"status":"timeout"`, "::end-spawn"} {
		if !strings.Contains(decorated, want) {
			t.Fatalf("decorated = %q, missing %q", decorated, want)
		}
	}

	// The record is consumed: a second pass leaves the (new) bare marker alone.
	if again := recorder.decorate(text); again != text {
		t.Fatalf("second decorate = %q, want untouched", again)
	}
}

func TestSpawnRecorderLeavesUnknownRunsBare(t *testing.T) {
	recorder := newSpawnRecorder()
	text := domain.SpawnMarker("run-desconhecido")
	if got := recorder.decorate(text); got != text {
		t.Fatalf("decorate = %q, want bare marker", got)
	}
}

func TestSpawnRecorderForget(t *testing.T) {
	recorder := newSpawnRecorder()
	recorder.observe("run-1", subagent.SpawnStatus{Phase: subagent.SpawnPhaseStart, Tasks: []subagent.SpawnTaskRef{{ID: "t1", Task: "x"}}})
	recorder.forget("run-1")
	text := domain.SpawnMarker("run-1")
	if got := recorder.decorate(text); got != text {
		t.Fatalf("decorate after forget = %q, want bare marker", got)
	}
}
