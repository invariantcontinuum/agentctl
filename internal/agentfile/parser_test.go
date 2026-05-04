package agentfile

import (
	"strings"
	"testing"
)

func TestParseAgentfile(t *testing.T) {
	input := `
AGENT planner-local
IMAGE planner:latest
TYPE planner
MODEL openai default endpoint=https://api.openai.com/v1 auth=api_key credential_env=OPENAI_API_KEY
SKILL ./skills/planner.md
MCP search http://localhost:9001/mcp
VECTOR docs pgvector postgres://localhost:5432/agentctl docs_chunks
GRAPH tasks neo4j bolt://localhost:7687
MEMORY session short window=12000
LOOP react max_steps=30
ENDPOINT http http://127.0.0.1:8088
ENV AGENTCTL_LOG_LEVEL=info
LABEL owner=platform
EXEC ["sh", "-c", "echo # not a comment"]
`

	config, err := NewParser().Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if config.Name != "planner-local" {
		t.Fatalf("Name = %q, want planner-local", config.Name)
	}
	if config.Type != "planner" {
		t.Fatalf("Type = %q, want planner", config.Type)
	}
	if config.Image != "planner:latest" {
		t.Fatalf("Image = %q, want planner:latest", config.Image)
	}
	if config.Model.Provider != "openai" {
		t.Fatalf("Model provider = %q, want openai", config.Model.Provider)
	}
	if config.Model.CredentialEnv != "OPENAI_API_KEY" {
		t.Fatalf("Model credential env = %q, want OPENAI_API_KEY", config.Model.CredentialEnv)
	}
	if got := config.Skills[0].Source; got != "./skills/planner.md" {
		t.Fatalf("Skill = %q, want ./skills/planner.md", got)
	}
	if got := config.MCPServers[0].Name; got != "search" {
		t.Fatalf("MCP name = %q, want search", got)
	}
	if got := config.VectorStores[0].Collection; got != "docs_chunks" {
		t.Fatalf("Vector collection = %q, want docs_chunks", got)
	}
	if got := config.GraphStores[0].Provider; got != "neo4j" {
		t.Fatalf("Graph provider = %q, want neo4j", got)
	}
	if got := config.Memories[0].Source; got != "window=12000" {
		t.Fatalf("Memory source = %q, want window=12000", got)
	}
	if config.Loop.MaxSteps != 30 {
		t.Fatalf("MaxSteps = %d, want 30", config.Loop.MaxSteps)
	}
	if got := config.Env["AGENTCTL_LOG_LEVEL"]; got != "info" {
		t.Fatalf("Env = %q, want info", got)
	}
	if got := config.Labels["owner"]; got != "platform" {
		t.Fatalf("Label = %q, want platform", got)
	}
	if got := strings.Join(config.Exec, " "); got != "sh -c echo # not a comment" {
		t.Fatalf("Exec = %q, want quoted comment preserved", got)
	}
}

func TestParseRejectsUnknownDirective(t *testing.T) {
	_, err := NewParser().Parse(strings.NewReader("UNKNOWN value\n"))
	if err == nil {
		t.Fatal("Parse returned nil error for unknown directive")
	}
}
