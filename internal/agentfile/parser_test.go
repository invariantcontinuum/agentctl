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
MODEL openai default base_url=https://api.openai.com/v1 auth=api_key api_key_env=OPENAI_API_KEY timeout_sec=30 option.temperature=0.2
SKILL ./skills/planner.md name=planner type=markdown depends_on=base metadata.owner=platform
SKILL inline content=Use_directives name=inline type=prompt
MCP search http http://localhost:9001/mcp header.Authorization=Bearer_test timeout_sec=5
MCP_TOOL search web web_search description=Search_the_web category=search metadata.cost=1
MCP fs stdio npx -y @modelcontextprotocol/server-filesystem /tmp
VECTOR docs pgvector postgres://localhost:5432/agentctl docs_chunks
GRAPH tasks neo4j bolt://localhost:7687
MEMORY session short inmemory limit=12000
LOOP react max_steps=30
HOOK pre audit http url=http://localhost:9010/pre timeout_sec=2 on_error=halt label.phase=pre
HOOK post summarize mcp timeout_sec=3 on_error=continue
EVALUATION max_errors=2 tool_allow_list=web,fs log_filter=secret completion_criteria=task_done,timeout
VALIDATOR_TOOL schema schema_check description=Validate category=validation
MULTI_AGENT enabled=true coordinator=coordinator allowed_roles=coder,reviewer delegation=policy policy.max_parallel=2
ENDPOINT http http://127.0.0.1:8088
ENDPOINT admin scheme=http host=127.0.0.1 port=9090 path=/admin label.scope=internal
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
	if config.Model.APIKeyEnv != "OPENAI_API_KEY" {
		t.Fatalf("Model API key env = %q, want OPENAI_API_KEY", config.Model.APIKeyEnv)
	}
	if config.Model.TimeoutSec != 30 || config.Model.Options["temperature"] != 0.2 {
		t.Fatalf("Model options not parsed: %+v", config.Model)
	}
	if got := config.Skills[0].Path; got != "./skills/planner.md" {
		t.Fatalf("Skill = %q, want ./skills/planner.md", got)
	}
	if got := config.Skills[0].Dependencies; len(got) != 1 || got[0] != "base" {
		t.Fatalf("Skill dependencies = %v, want [base]", got)
	}
	if config.Skills[1].Content != "Use_directives" || config.Skills[1].Path != "" {
		t.Fatalf("Inline skill = %+v", config.Skills[1])
	}
	if got := config.MCPServers[0].Name; got != "search" {
		t.Fatalf("MCP name = %q, want search", got)
	}
	if got := config.MCPServers[0].URL; got != "http://localhost:9001/mcp" {
		t.Fatalf("MCP[0] URL = %q, want http://localhost:9001/mcp", got)
	}
	if got := config.MCPServers[0].Headers["Authorization"]; got != "Bearer_test" {
		t.Fatalf("MCP[0] header = %q, want Bearer_test", got)
	}
	if got := config.MCPServers[0].Tools[0].Category; got != "search" {
		t.Fatalf("MCP tool category = %q, want search", got)
	}
	if got := config.MCPServers[1].URL; got != "" {
		t.Fatalf("MCP[1] URL = %q, want empty", got)
	}
	if got := config.MCPServers[1].Command; got != "npx" {
		t.Fatalf("MCP[1] Command = %q, want npx", got)
	}
	if got := strings.Join(config.MCPServers[1].Args, " "); got != "-y @modelcontextprotocol/server-filesystem /tmp" {
		t.Fatalf("MCP[1] Args = %q, want -y @modelcontextprotocol/server-filesystem /tmp", got)
	}
	if got := config.VectorStores[0].Index; got != "docs_chunks" {
		t.Fatalf("Vector index = %q, want docs_chunks", got)
	}
	if got := config.GraphStores[0].Provider; got != "neo4j" {
		t.Fatalf("Graph provider = %q, want neo4j", got)
	}
	if got := config.Memories[0].Limit; got != 12000 {
		t.Fatalf("Memory limit = %d, want 12000", got)
	}
	if config.Loop.MaxSteps != 30 {
		t.Fatalf("MaxSteps = %d, want 30", config.Loop.MaxSteps)
	}
	if got := config.Loop.PreHooks[0].Name; got != "audit" {
		t.Fatalf("PreHooks[0] = %q, want audit", got)
	}
	if got := config.Loop.Evaluation.ToolAllowList; len(got) != 2 || got[0] != "web" || got[1] != "fs" {
		t.Fatalf("ToolAllowList = %v, want [web fs]", got)
	}
	if got := config.Loop.Evaluation.ValidatorTools[0].Name; got != "schema_check" {
		t.Fatalf("Validator tool = %q, want schema_check", got)
	}
	if !config.Loop.MultiAgent.Enabled || config.Loop.MultiAgent.Coordinator != "coordinator" {
		t.Fatalf("MultiAgent = %+v", config.Loop.MultiAgent)
	}
	if got := config.Endpoints[1].Path; got != "/admin" {
		t.Fatalf("Endpoint path = %q, want /admin", got)
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
