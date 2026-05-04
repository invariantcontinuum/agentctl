package agentfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile %s returned error: %v", path, err)
	}
}

func TestFromInheritsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	parentPath := filepath.Join(dir, "Base")
	childPath := filepath.Join(dir, "Agentfile")

	writeFile(t, parentPath, `
AGENT base
TYPE planner
MODEL anthropic default base_url=https://api.anthropic.com auth=api_key api_key_env=ANTHROPIC_API_KEY
SKILL ./skills/planner.md
MCP search http http://localhost:9001/mcp
LOOP react max_steps=20
ENV BASE_KEY=base
LABEL owner=base
EXEC ["sh", "-c", "echo base"]
`)

	writeFile(t, childPath, `
FROM ./Base
AGENT child
SKILL ./skills/child.md
MCP fs stdio npx -y @modelcontextprotocol/server-filesystem /tmp
LOOP react max_steps=40
ENV CHILD_KEY=child
LABEL owner=child
`)

	config, err := NewParser().ParseFile(childPath)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if config.Name != "child" {
		t.Fatalf("Name = %q, want child (override)", config.Name)
	}
	if config.Type != "planner" {
		t.Fatalf("Type = %q, want planner (inherited)", config.Type)
	}
	if config.Model.Provider != "anthropic" {
		t.Fatalf("Model.Provider = %q, want anthropic (inherited)", config.Model.Provider)
	}
	if got := len(config.Skills); got != 2 {
		t.Fatalf("len(Skills) = %d, want 2 (parent+child append)", got)
	}
	if got := len(config.MCPServers); got != 2 {
		t.Fatalf("len(MCPServers) = %d, want 2 (parent http + child stdio)", got)
	}
	if config.Loop.MaxSteps != 40 {
		t.Fatalf("Loop.MaxSteps = %d, want 40 (override)", config.Loop.MaxSteps)
	}
	if config.Env["BASE_KEY"] != "base" || config.Env["CHILD_KEY"] != "child" {
		t.Fatalf("Env = %v, want both keys", config.Env)
	}
	if config.Labels["owner"] != "child" {
		t.Fatalf("Labels[owner] = %q, want child (override)", config.Labels["owner"])
	}
	if got := strings.Join(config.Exec, " "); got != "sh -c echo base" {
		t.Fatalf("Exec = %q, want inherited base exec", got)
	}
}

func TestFromCycleRejected(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A")
	b := filepath.Join(dir, "B")
	writeFile(t, a, "FROM ./B\nAGENT a")
	writeFile(t, b, "FROM ./A\nAGENT b")

	if _, err := NewParser().ParseFile(a); err == nil || !strings.Contains(err.Error(), "FROM cycle") {
		t.Fatalf("err = %v, want FROM cycle error", err)
	}
}

func TestFromInStreamParseRejected(t *testing.T) {
	body := strings.NewReader("FROM ./parent\nAGENT child")
	if _, err := NewParser().Parse(body); err == nil || !strings.Contains(err.Error(), "FROM is only valid") {
		t.Fatalf("err = %v, want stream parse rejection", err)
	}
}
