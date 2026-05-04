package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

func TestJSONRepositorySaveFindAndList(t *testing.T) {
	repo := NewJSONRepository(filepath.Join(t.TempDir(), "state.json"))
	now := time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC)

	instance := Instance{
		ID:        "planner-1",
		Name:      "planner",
		Type:      "planner",
		Status:    "running",
		Config:    agent.Config{Name: "planner", Type: "planner"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.Save(instance); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	found, err := repo.Find("planner-1")
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if found.Name != "planner" {
		t.Fatalf("Name = %q, want planner", found.Name)
	}

	list, err := repo.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List length = %d, want 1", len(list))
	}

	if err := repo.Delete("planner-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := repo.Find("planner-1"); err == nil {
		t.Fatal("Find returned nil error after Delete")
	}
}
