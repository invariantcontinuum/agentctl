package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/store"
)

func writeAgentLog(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func TestLogsLevelFilterDropsBelowMin(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "coder.log")
	writeAgentLog(t, logPath, `
{"ts":"2026-05-04T10:00:00Z","level":"debug","msg":"connecting"}
{"ts":"2026-05-04T10:00:01Z","level":"info","msg":"started"}
{"ts":"2026-05-04T10:00:02Z","level":"warn","msg":"slow query"}
{"ts":"2026-05-04T10:00:03Z","level":"error","msg":"oops","fields":{"err":"boom"}}
`)

	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{
		ID:        "coder-1",
		Type:      "coder",
		LogPath:   logPath,
		Config:    agent.Config{Name: "coder", Type: "coder", Loop: agent.Loop{Name: "react", MaxSteps: 1}, Exec: []string{"sleep", "1"}},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{})

	exitCode := app.Run(context.Background(), []string{"logs", "--level", "warn", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	output := out.String()
	if strings.Contains(output, "started") || strings.Contains(output, "connecting") {
		t.Fatalf("output leaked below-min lines: %s", output)
	}
	if !strings.Contains(output, "WARN slow query") {
		t.Fatalf("output missing warn line: %s", output)
	}
	if !strings.Contains(output, "ERROR oops err=boom") {
		t.Fatalf("output missing error fields: %s", output)
	}
}

func TestLogsRejectsUnknownLevel(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "coder.log")
	writeAgentLog(t, logPath, `{"level":"info","msg":"hi"}`)

	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{
		ID:        "coder-1",
		Type:      "coder",
		LogPath:   logPath,
		Config:    agent.Config{Name: "coder", Type: "coder", Loop: agent.Loop{Name: "react", MaxSteps: 1}, Exec: []string{"sleep", "1"}},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{})

	exitCode := app.Run(context.Background(), []string{"logs", "--level", "trace", "coder-1"})
	if exitCode == 0 {
		t.Fatal("exitCode = 0, want failure for unknown level")
	}
}
