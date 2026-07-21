package application

import "testing"

func TestChatRunCoordinatorStopReleasesSlotImmediately(t *testing.T) {
	var coordinator ChatRunCoordinator
	canceled := false
	token1, err := coordinator.BeginRun("chat-1", func() { canceled = true })
	if err != nil {
		t.Fatalf("BeginRun(first) error = %v", err)
	}

	coordinator.Stop("chat-1")
	if !canceled {
		t.Fatal("Stop did not call cancel")
	}
	if coordinator.IsRunning("chat-1") {
		t.Fatal("Stop should release the chat slot immediately")
	}

	token2, err := coordinator.BeginRun("chat-1", func() {})
	if err != nil {
		t.Fatalf("BeginRun(second) error = %v", err)
	}
	if token2 == token1 {
		t.Fatal("new run reused old token")
	}

	coordinator.EndRun("chat-1", token1)
	if !coordinator.IsRunning("chat-1") {
		t.Fatal("stale EndRun cleared the newer run")
	}
	coordinator.EndRun("chat-1", token2)
	if coordinator.IsRunning("chat-1") {
		t.Fatal("matching EndRun did not clear the run")
	}
}

func TestChatRunCoordinatorStreamingIsTokenGuarded(t *testing.T) {
	var coordinator ChatRunCoordinator
	token1, err := coordinator.BeginRun("chat-1", func() {})
	if err != nil {
		t.Fatalf("BeginRun(first) error = %v", err)
	}
	coordinator.SetStreamingRun("chat-1", token1)

	coordinator.Stop("chat-1")
	token2, err := coordinator.BeginRun("chat-1", func() {})
	if err != nil {
		t.Fatalf("BeginRun(second) error = %v", err)
	}
	coordinator.SetStreamingRun("chat-1", token2)

	coordinator.ClearStreamingRun(token1)
	if got := coordinator.Streaming(); got != "chat-1" {
		t.Fatalf("stale ClearStreamingRun cleared newer streaming chat, got %q", got)
	}
	coordinator.ClearStreamingRun(token2)
	if got := coordinator.Streaming(); got != "" {
		t.Fatalf("matching ClearStreamingRun did not clear streaming chat, got %q", got)
	}
}

func TestChatRunCoordinatorBeginCompatibility(t *testing.T) {
	var coordinator ChatRunCoordinator
	if err := coordinator.Begin("chat-1", func() {}); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := coordinator.Begin("chat-1", func() {}); err != ErrChatAlreadyRunning {
		t.Fatalf("Begin() while running = %v, want ErrChatAlreadyRunning", err)
	}
	coordinator.End("chat-1")
	if coordinator.IsRunning("chat-1") {
		t.Fatal("End did not clear run")
	}
	coordinator.Stop("chat-1")
}
