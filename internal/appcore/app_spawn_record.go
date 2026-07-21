package appcore

import (
	"sync"

	"aw/internal/domain"
	"aw/internal/infrastructure/subagent"
)

// spawnRecorder collects each run's system.spawn task labels and results from
// the lifecycle statuses (the same ones broadcast to the chat UI), so the
// assistant reply can be persisted with the results embedded at its ::spawn
// marker — the chat card then survives chat reload and app restart.
type spawnRecorder struct {
	mu      sync.Mutex
	records map[string]*spawnRecord
}

type spawnRecord struct {
	order   []string
	labels  map[string]string
	results map[string]subagent.SpawnResult
}

func newSpawnRecorder() *spawnRecorder {
	return &spawnRecorder{records: map[string]*spawnRecord{}}
}

func (r *spawnRecorder) observe(runID string, status subagent.SpawnStatus) {
	// nil-safe: tests build App by struct literal, without NewApp.
	if r == nil || runID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	record := r.records[runID]
	if record == nil {
		record = &spawnRecord{labels: map[string]string{}, results: map[string]subagent.SpawnResult{}}
		r.records[runID] = record
	}
	for _, task := range status.Tasks {
		if _, seen := record.labels[task.ID]; !seen {
			record.order = append(record.order, task.ID)
		}
		record.labels[task.ID] = task.Task
	}
	if status.Result != nil {
		record.results[status.Result.ID] = *status.Result
	}
	for _, result := range status.Results {
		record.results[result.ID] = result
	}
}

// decorate upgrades each bare ::spawn marker in the reply to its results block
// and drops the consumed record. Markers whose run was never observed stay
// bare (the frontend hides them).
func (r *spawnRecorder) decorate(text string) string {
	if r == nil {
		return text
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return domain.ReplaceSpawnMarkers(text, func(runID string) (string, bool) {
		record := r.records[runID]
		if record == nil || len(record.order) == 0 {
			return "", false
		}
		tasks := make([]domain.SpawnCardTask, 0, len(record.order))
		for _, id := range record.order {
			task := domain.SpawnCardTask{ID: id, Task: record.labels[id], Status: "running"}
			if result, ok := record.results[id]; ok {
				task.Status = result.Status
				task.Output = result.Output
				task.ElapsedMs = result.ElapsedMs
			}
			tasks = append(tasks, task)
		}
		block, err := domain.SpawnResultsBlock(runID, tasks)
		if err != nil {
			return "", false
		}
		delete(r.records, runID)
		return block, true
	})
}

// forget drops a run's record (end-of-run cleanup for replies that never got
// persisted, e.g. stopped runs).
func (r *spawnRecorder) forget(runID string) {
	if r == nil || runID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.records, runID)
}
