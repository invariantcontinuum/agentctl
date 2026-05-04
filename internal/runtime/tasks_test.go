package runtime

import (
	"errors"
	"testing"
)

func TestStoreSubmitProducesSnapshotAndQueuesTask(t *testing.T) {
	store := NewStore(2, 0)
	task, err := store.Submit("hello", "system")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Status != StatusQueued {
		t.Fatalf("status = %s, want %s", task.Status, StatusQueued)
	}
	if task.Prompt != "hello" || task.System != "system" {
		t.Fatalf("payload not preserved: %+v", task)
	}

	got, ok := store.Next()
	if !ok || got.ID != task.ID {
		t.Fatalf("Next did not return submitted task: got=%+v ok=%v", got, ok)
	}
}

func TestStoreSubmitRejectsEmptyPrompt(t *testing.T) {
	store := NewStore(1, 0)
	if _, err := store.Submit("", ""); err == nil {
		t.Fatalf("expected error for empty prompt")
	}
}

func TestStoreLifecycleTransitions(t *testing.T) {
	store := NewStore(1, 0)
	task, err := store.Submit("p", "")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := store.MarkRunning(task.ID); err != nil {
		t.Fatalf("running: %v", err)
	}
	if err := store.MarkDone(task.ID, "result"); err != nil {
		t.Fatalf("done: %v", err)
	}
	final, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if final.Status != StatusDone || final.Result != "result" {
		t.Fatalf("unexpected final: %+v", final)
	}
	if final.StartedAt.IsZero() || final.CompletedAt.IsZero() {
		t.Fatalf("timestamps not recorded: %+v", final)
	}
}

func TestStoreMarkErrorRecordsMessage(t *testing.T) {
	store := NewStore(1, 0)
	task, err := store.Submit("p", "")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := store.MarkError(task.ID, errors.New("boom")); err != nil {
		t.Fatalf("error: %v", err)
	}
	final, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if final.Status != StatusError || final.Error != "boom" {
		t.Fatalf("unexpected: %+v", final)
	}
}

func TestStoreCountsAndListAreOrdered(t *testing.T) {
	store := NewStore(4, 0)
	for _, prompt := range []string{"a", "b", "c"} {
		if _, err := store.Submit(prompt, ""); err != nil {
			t.Fatalf("submit %s: %v", prompt, err)
		}
	}
	counts := store.Counts()
	if counts[StatusQueued] != 3 {
		t.Fatalf("queued = %d, want 3", counts[StatusQueued])
	}

	list := store.List()
	if len(list) != 3 {
		t.Fatalf("list len = %d, want 3", len(list))
	}
	if list[0].Prompt != "a" || list[2].Prompt != "c" {
		t.Fatalf("list not in submission order: %+v", list)
	}
}

func TestStoreGetUnknownReturnsNotFound(t *testing.T) {
	store := NewStore(1, 0)
	if _, err := store.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
