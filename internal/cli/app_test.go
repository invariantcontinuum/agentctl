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
	"github.com/invariantcontinuum/agentctl/internal/driver"
	"github.com/invariantcontinuum/agentctl/internal/store"
)

func TestRunDryRunPrintsParsedConfig(t *testing.T) {
	dir := t.TempDir()
	agentfilePath := filepath.Join(dir, "Agentfile")
	writeTestFile(t, agentfilePath, `
AGENT planner
TYPE planner
LOOP react max_steps=5
EXEC ["sleep", "60"]
`)

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, store.NewJSONRepository(filepath.Join(dir, "state.json")), fakeDriver{})
	app.paths = testRuntimePaths(dir)

	exitCode := app.Run(context.Background(), []string{"run", "-f", agentfilePath, "--dry-run"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), `"name": "planner"`) {
		t.Fatalf("stdout did not contain parsed name: %s", out.String())
	}
}

func TestRunStoresStartedAgent(t *testing.T) {
	dir := t.TempDir()
	agentfilePath := filepath.Join(dir, "Agentfile")
	writeTestFile(t, agentfilePath, `
AGENT planner
TYPE planner
LOOP react max_steps=5
EXEC ["sleep", "60"]
`)

	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})
	app.now = func() time.Time { return time.Unix(100, 0).UTC() }
	app.paths = testRuntimePaths(dir)

	exitCode := app.Run(context.Background(), []string{"run", "-f", agentfilePath})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}

	id := strings.TrimSpace(out.String())
	instance, err := repo.Find(id)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if instance.PID != 42 {
		t.Fatalf("PID = %d, want 42", instance.PID)
	}
}

func TestRunImageDryRunUsesCatalog(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, store.NewJSONRepository(filepath.Join(dir, "state.json")), fakeDriver{})
	app.paths = testRuntimePaths(dir)

	exitCode := app.Run(context.Background(), []string{"run", "--dry-run", "coder:latest"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), `"image": "coder:latest"`) {
		t.Fatalf("stdout did not contain image: %s", out.String())
	}
	if !strings.Contains(out.String(), `"type": "coder"`) {
		t.Fatalf("stdout did not contain coder type: %s", out.String())
	}
}

func TestPsQuietAllPrintsIDsOnly(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{ID: "coder-1", Type: "coder", Status: "running", PID: 42, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := repo.Save(store.Instance{ID: "reviewer-1", Type: "reviewer", Status: "stopped", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"ps", "-aq"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if got := out.String(); got != "coder-1\nreviewer-1\n" {
		t.Fatalf("stdout = %q, want IDs only", got)
	}
}

func TestAgentsLsDelegatesToPs(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{ID: "planner-1", Image: "planner:latest", Type: "planner", Status: "running", PID: 42, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"agents", "ls"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "planner-1") {
		t.Fatalf("stdout did not contain planner-1: %s", out.String())
	}
}

func TestModelsLsListsProviders(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, store.NewJSONRepository(filepath.Join(dir, "state.json")), fakeDriver{})

	exitCode := app.Run(context.Background(), []string{"models", "ls"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	for _, want := range []string{"openai:default", "anthropic:default", "gemini:default", "vllm:local", "llamacpp:local"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout did not contain %q: %s", want, out.String())
		}
	}
}

func TestRunRmDeletesStateOnStop(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})
	app.now = func() time.Time { return time.Unix(100, 0).UTC() }
	app.paths = testRuntimePaths(dir)

	exitCode := app.Run(context.Background(), []string{"run", "--rm", "coder:latest"})
	if exitCode != 0 {
		t.Fatalf("run exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	id := strings.TrimSpace(out.String())

	exitCode = app.Run(context.Background(), []string{"stop", id})
	if exitCode != 0 {
		t.Fatalf("stop exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if _, err := repo.Find(id); err == nil {
		t.Fatal("Find returned nil error after --rm stop")
	}
}

func TestRmDeletesStoppedAgentStateAndFiles(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "agent.log")
	tracePath := filepath.Join(dir, "agent.trace")
	writeTestFile(t, logPath, "log")
	writeTestFile(t, tracePath, "trace")

	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{
		ID:        "coder-1",
		Image:     "coder:latest",
		Type:      "coder",
		Status:    "stopped",
		LogPath:   logPath,
		TracePath: tracePath,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{})

	exitCode := app.Run(context.Background(), []string{"rm", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if got := out.String(); got != "coder-1\n" {
		t.Fatalf("stdout = %q, want removed id", got)
	}
	if _, err := repo.Find("coder-1"); err == nil {
		t.Fatal("Find returned nil error after rm")
	}
	assertNotExists(t, logPath)
	assertNotExists(t, tracePath)
}

func TestRmRejectsRunningAgentWithoutForce(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{ID: "coder-1", Type: "coder", Status: "running", PID: 42, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"rm", "coder-1"})
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want failure", exitCode)
	}
	if !strings.Contains(errOut.String(), "without -f") {
		t.Fatalf("stderr did not mention -f: %s", errOut.String())
	}
	if _, err := repo.Find("coder-1"); err != nil {
		t.Fatalf("Find returned error after rejected rm: %v", err)
	}
}

func TestAgentsRmForceDeletesRunningAgent(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{ID: "coder-1", Type: "coder", Status: "running", PID: 42, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"agents", "rm", "-f", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if got := out.String(); got != "coder-1\n" {
		t.Fatalf("stdout = %q, want removed id", got)
	}
	if _, err := repo.Find("coder-1"); err == nil {
		t.Fatal("Find returned nil error after forced rm")
	}
}

func TestDescribePrintsHumanReadableDetails(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	config := agent.Config{
		Image: "coder:latest",
		Name:  "coder",
		Type:  "coder",
		Model: agent.Model{
			Provider: "vllm",
			Name:     "local",
			BaseURL:  "http://localhost:8000/v1",
			Auth:     "none",
		},
		Skills: []agent.Skill{{Name: "coder", Type: "builtin", Path: "builtin://skills/coder", Enabled: true}},
		Loop:   agent.Loop{Name: "react", MaxSteps: 30},
		Labels: map[string]string{"agentctl.taxonomy.control": "planning,loop,evaluation"},
	}
	if err := repo.Save(store.Instance{
		ID:        "coder-1",
		Name:      "coder",
		Image:     "coder:latest",
		Type:      "coder",
		Status:    "running",
		PID:       42,
		Config:    config,
		WorkDir:   ".",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})

	exitCode := app.Run(context.Background(), []string{"agents", "describe", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	for _, want := range []string{"Agent:", "ID: coder-1", "Image: coder:latest", "Model:", "Provider: vllm", "Skills:", "builtin://skills/coder", "Labels:", "agentctl.taxonomy.control=planning,loop,evaluation"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout did not contain %q: %s", want, out.String())
		}
	}
}

type fakeDriver struct {
	pid int
}

func (d fakeDriver) Start(context.Context, agent.Config, driver.StartOptions) (driver.Process, error) {
	if d.pid == 0 {
		d.pid = 1
	}
	return driver.Process{PID: d.pid}, nil
}

func (fakeDriver) Stop(context.Context, driver.Process) error {
	return nil
}

func (fakeDriver) Status(context.Context, driver.Process) (driver.Status, error) {
	return driver.StatusRunning, nil
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists or stat returned unexpected error: %v", path, err)
	}
}

func testRuntimePaths(dir string) func(string) (string, string, string, error) {
	return func(id string) (string, string, string, error) {
		return filepath.Join(dir, id+".log"),
			filepath.Join(dir, id+".trace"),
			filepath.Join(dir, id+".json"),
			nil
	}
}
