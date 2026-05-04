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

func testRuntimePaths(dir string) func(string) (string, string, error) {
	return func(id string) (string, string, error) {
		return filepath.Join(dir, id+".log"), filepath.Join(dir, id+".trace"), nil
	}
}
