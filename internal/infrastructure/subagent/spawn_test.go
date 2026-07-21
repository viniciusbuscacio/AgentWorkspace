package subagent

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aw/internal/domain"
	"aw/internal/infrastructure/tools"

	"google.golang.org/adk/tool"
)

// fakeRunner records peak concurrency and returns a per-task reply.
type fakeRunner struct {
	mu      sync.Mutex
	active  int
	peak    int
	blockCh chan struct{} // when non-nil, each run blocks until closed
	reply   func(text string) (string, error)
}

func (f *fakeRunner) RunIsolated(ctx context.Context, _ domain.ModelConfig, text string, _ string, _ []tool.Tool, _ int32) (domain.AgentReply, error) {
	f.mu.Lock()
	f.active++
	if f.active > f.peak {
		f.peak = f.active
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()
	if f.blockCh != nil {
		select {
		case <-f.blockCh:
		case <-ctx.Done():
			return domain.AgentReply{}, ctx.Err()
		}
	}
	out, err := f.reply(text)
	return domain.AgentReply{Text: out}, err
}

func testBase(t *testing.T) tools.Options {
	return tools.Options{Root: t.TempDir()}
}

func testDeps(runner IsolatedRunner, notify func(SpawnStatus)) Deps {
	return Deps{
		Runtime:     runner,
		ModelConfig: func() (domain.ModelConfig, error) { return domain.ModelConfig{}, nil },
		NotifySpawn: func(_ context.Context, s SpawnStatus) { notify(s) },
	}
}

func collectPhases(statuses []SpawnStatus) map[string]int {
	counts := map[string]int{}
	for _, s := range statuses {
		counts[s.Phase]++
	}
	return counts
}

func TestRunSpawnRunsTasksInParallelAndReportsLifecycle(t *testing.T) {
	runner := &fakeRunner{
		blockCh: make(chan struct{}),
		reply:   func(text string) (string, error) { return "done: " + text, nil },
	}
	// Release all workers only once the third has entered, proving parallelism.
	go func() {
		for {
			runner.mu.Lock()
			p := runner.peak
			runner.mu.Unlock()
			if p >= 3 {
				close(runner.blockCh)
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	var mu sync.Mutex
	var statuses []SpawnStatus
	notify := func(s SpawnStatus) { mu.Lock(); statuses = append(statuses, s); mu.Unlock() }

	tasks := []SpawnTask{{ID: "a", Task: "task A"}, {ID: "b", Task: "task B"}, {ID: "c", Task: "task C"}}
	results, err := RunSpawn(context.Background(), testDeps(runner, notify), tasks, time.Minute, testBase(t))
	if err != nil {
		t.Fatalf("RunSpawn: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("want 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != SpawnStatusSuccess {
			t.Fatalf("task %s: want success, got %s (%s)", r.ID, r.Status, r.Output)
		}
		if !strings.HasPrefix(r.Output, "done:") {
			t.Fatalf("task %s: unexpected output %q", r.ID, r.Output)
		}
	}
	if runner.peak < 3 {
		t.Fatalf("want peak concurrency 3, got %d", runner.peak)
	}
	phases := collectPhases(statuses)
	if phases[SpawnPhaseStart] != 1 || phases[SpawnPhaseTaskDone] != 3 || phases[SpawnPhaseEnd] != 1 {
		t.Fatalf("unexpected phase counts: %+v", phases)
	}
}

func TestRunGenericTaskTimeoutMarksTaskTimedOut(t *testing.T) {
	// Exercise runGenericTask directly with a sub-floor timeout so the deadline
	// path is fast; RunSpawn floors the timeout to MinSpawnTimeout in production.
	runner := &fakeRunner{
		blockCh: make(chan struct{}), // never closed -> worker hits the deadline
		reply:   func(string) (string, error) { return "unreachable", nil },
	}
	res := runGenericTask(context.Background(), testDeps(runner, func(SpawnStatus) {}),
		SpawnTask{ID: "x", Task: "slow"}, 20*time.Millisecond, testBase(t))
	if res.Status != SpawnStatusTimeout {
		t.Fatalf("want timeout result, got %+v", res)
	}
}

func TestRunSpawnFloorsTinyTimeout(t *testing.T) {
	runner := &fakeRunner{reply: func(string) (string, error) { return "ok", nil }}
	// A 1ms timeout must be floored (not instantly time out) so the quick task
	// completes with success.
	results, err := RunSpawn(context.Background(), testDeps(runner, func(SpawnStatus) {}),
		[]SpawnTask{{ID: "x", Task: "quick"}}, time.Millisecond, testBase(t))
	if err != nil {
		t.Fatalf("RunSpawn: %v", err)
	}
	if len(results) != 1 || results[0].Status != SpawnStatusSuccess {
		t.Fatalf("want floored timeout to allow success, got %+v", results)
	}
}

func TestRunSpawnRejectsTooManyTasks(t *testing.T) {
	runner := &fakeRunner{reply: func(string) (string, error) { return "", nil }}
	tasks := make([]SpawnTask, MaxConcurrentSpawn+1)
	for i := range tasks {
		tasks[i] = SpawnTask{Task: "t"}
	}
	if _, err := RunSpawn(context.Background(), testDeps(runner, func(SpawnStatus) {}), tasks, 0, testBase(t)); err == nil {
		t.Fatal("want error for exceeding the concurrency cap")
	}
}

func TestRunSpawnAssignsMissingIDsAndDropsEmpty(t *testing.T) {
	var ran int32
	runner := &fakeRunner{reply: func(string) (string, error) { atomic.AddInt32(&ran, 1); return "ok", nil }}
	tasks := []SpawnTask{{Task: "no id here"}, {Task: "   "}, {ID: "keep", Task: "has id"}}
	results, err := RunSpawn(context.Background(), testDeps(runner, func(SpawnStatus) {}), tasks, time.Minute, testBase(t))
	if err != nil {
		t.Fatalf("RunSpawn: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results (empty dropped), got %d", len(results))
	}
	if atomic.LoadInt32(&ran) != 2 {
		t.Fatalf("want 2 runs, got %d", ran)
	}
	for _, r := range results {
		if strings.TrimSpace(r.ID) == "" {
			t.Fatalf("result missing id: %+v", r)
		}
	}
}
