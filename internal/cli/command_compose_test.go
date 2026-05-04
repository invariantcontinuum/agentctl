package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/store"
)

func writeTeamCompose(t *testing.T, dir string) (string, string, string) {
	t.Helper()
	plannerPath := filepath.Join(dir, "planner", "Agentfile")
	if err := os.MkdirAll(filepath.Dir(plannerPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	writeTestFile(t, plannerPath, `
AGENT planner
TYPE planner
LOOP react max_steps=5
EXEC ["sleep", "60"]
`)

	coderPath := filepath.Join(dir, "coder", "Agentfile")
	if err := os.MkdirAll(filepath.Dir(coderPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	writeTestFile(t, coderPath, `
AGENT coder
TYPE coder
LOOP react max_steps=5
EXEC ["sleep", "60"]
`)

	composePath := filepath.Join(dir, "AgentCompose")
	writeTestFile(t, composePath, `
COMPOSE smoke-team
AGENT planner FILE=./planner/Agentfile
AGENT coder FILE=./coder/Agentfile DEPENDS_ON=planner
`)
	return composePath, plannerPath, coderPath
}

func TestComposeUpStartsServicesInOrder(t *testing.T) {
	dir := t.TempDir()
	composePath, _, _ := writeTeamCompose(t, dir)

	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})
	app.now = func() time.Time { return time.Unix(100, 0).UTC() }
	app.paths = testRuntimePaths(dir)

	exitCode := app.Run(context.Background(), []string{"compose", "up", "-f", composePath})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}

	output := out.String()
	plannerIndex := strings.Index(output, "planner\t")
	coderIndex := strings.Index(output, "coder\t")
	if plannerIndex < 0 || coderIndex < 0 {
		t.Fatalf("missing service lines in output: %s", output)
	}
	if plannerIndex > coderIndex {
		t.Fatalf("planner started after coder despite DEPENDS_ON: %s", output)
	}

	instances, err := repo.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("len(instances) = %d, want 2", len(instances))
	}
	for _, instance := range instances {
		if instance.Config.Labels["agentctl.compose.project"] != "smoke-team" {
			t.Fatalf("instance %s missing compose project label: %+v", instance.ID, instance.Config.Labels)
		}
	}
}

func TestComposeDownStopsAndRemoves(t *testing.T) {
	dir := t.TempDir()
	composePath, _, _ := writeTeamCompose(t, dir)

	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})
	app.now = func() time.Time { return time.Unix(100, 0).UTC() }
	app.paths = testRuntimePaths(dir)

	if exitCode := app.Run(context.Background(), []string{"compose", "up", "-f", composePath}); exitCode != 0 {
		t.Fatalf("compose up exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if exitCode := app.Run(context.Background(), []string{"compose", "down", "-f", composePath}); exitCode != 0 {
		t.Fatalf("compose down exitCode = %d, stderr = %s", exitCode, errOut.String())
	}

	instances, err := repo.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(instances) != 0 {
		t.Fatalf("len(instances) = %d after compose down, want 0", len(instances))
	}
}

func TestComposeLsPrintsPlan(t *testing.T) {
	dir := t.TempDir()
	composePath, _, _ := writeTeamCompose(t, dir)

	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{})

	exitCode := app.Run(context.Background(), []string{"compose", "ls", "-f", composePath})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	output := out.String()
	if !strings.Contains(output, "PROJECT smoke-team") {
		t.Fatalf("output missing project line: %s", output)
	}
	if !strings.Contains(output, "planner") || !strings.Contains(output, "coder") {
		t.Fatalf("output missing service lines: %s", output)
	}
}
