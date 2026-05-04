package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/store"
)

func savedAgent(t *testing.T, dir string, mutate func(*store.Instance)) (*store.JSONRepository, store.Instance) {
	t.Helper()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	instance := store.Instance{
		ID:     "coder-1",
		Type:   "coder",
		Status: "running",
		PID:    42,
		Config: agent.Config{
			Name:         "coder",
			Type:         "coder",
			VectorStores: []agent.RAGSource{{Name: "docs", Type: "vector", Provider: "pgvector", URL: "postgres://x", Index: "docs_chunks"}},
			GraphStores:  []agent.RAGSource{{Name: "tasks", Type: "graph", Provider: "neo4j", URL: "bolt://x"}},
			Memories:     []agent.Memory{{Name: "session", Type: "short", Provider: "inmemory", Limit: 8000}, {Name: "plans", Type: "long", Provider: "postgres", Bucket: "tasks"}},
			Loop:         agent.Loop{Name: "react", MaxSteps: 30},
			Exec:         []string{"sleep", "1"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if mutate != nil {
		mutate(&instance)
	}
	if err := repo.Save(instance); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	return repo, instance
}

func TestRagListPrintsVectorAndGraph(t *testing.T) {
	dir := t.TempDir()
	repo, _ := savedAgent(t, dir, nil)

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"rag", "ls", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	output := out.String()
	if !strings.Contains(output, "VECTOR  docs") {
		t.Fatalf("output missing vector line: %s", output)
	}
	if !strings.Contains(output, "GRAPH   tasks") {
		t.Fatalf("output missing graph line: %s", output)
	}
}

func TestRagVectorListFiltersToVector(t *testing.T) {
	dir := t.TempDir()
	repo, _ := savedAgent(t, dir, nil)

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"rag", "vector", "ls", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if strings.Contains(out.String(), "GRAPH") {
		t.Fatalf("vector ls printed GRAPH line: %s", out.String())
	}
}

func TestMemoryListAll(t *testing.T) {
	dir := t.TempDir()
	repo, _ := savedAgent(t, dir, nil)

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"memory", "ls", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	output := out.String()
	if !strings.Contains(output, "session") || !strings.Contains(output, "plans") {
		t.Fatalf("memory ls missing entries: %s", output)
	}
}

func TestMemoryShortListFiltersToShort(t *testing.T) {
	dir := t.TempDir()
	repo, _ := savedAgent(t, dir, nil)

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"memory", "short", "ls", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if strings.Contains(out.String(), "long") {
		t.Fatalf("memory short ls returned long-kind binding: %s", out.String())
	}
}

func TestMemoryRecallReturnsBindingDetails(t *testing.T) {
	dir := t.TempDir()
	repo, _ := savedAgent(t, dir, nil)

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"memory", "recall", "coder-1", "plans"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "plans\tlong\ttasks") {
		t.Fatalf("memory recall did not print binding row: %s", out.String())
	}
}

func TestLoopListShowsLoopName(t *testing.T) {
	dir := t.TempDir()
	repo, _ := savedAgent(t, dir, nil)

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"loop", "ls"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "react") {
		t.Fatalf("loop ls missing react loop name: %s", out.String())
	}
}

func TestGuardLsReturnsPlaceholder(t *testing.T) {
	dir := t.TempDir()
	repo, _ := savedAgent(t, dir, nil)

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"guard", "ls", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "no guardrails configured") {
		t.Fatalf("guard ls missing placeholder: %s", out.String())
	}
}

func TestSingularAgentAndModelAliases(t *testing.T) {
	dir := t.TempDir()
	repo, _ := savedAgent(t, dir, nil)

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	if exitCode := app.Run(context.Background(), []string{"agent", "ls"}); exitCode != 0 {
		t.Fatalf("agent ls exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "coder-1") {
		t.Fatalf("agent ls missing instance: %s", out.String())
	}

	out.Reset()
	if exitCode := app.Run(context.Background(), []string{"model", "ls"}); exitCode != 0 {
		t.Fatalf("model ls exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "openai:default") {
		t.Fatalf("model ls missing openai provider: %s", out.String())
	}
}
