package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/mcp"
	"github.com/invariantcontinuum/agentctl/internal/store"
)

func readAllFromPath(path string) ([]byte, error) {
	return os.ReadFile(path)
}

type stubMCPHTTP struct {
	respond func(*http.Request) (*http.Response, error)
}

func (s stubMCPHTTP) Do(request *http.Request) (*http.Response, error) {
	return s.respond(request)
}

func mcpJSON(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestToolMCPListsDiscoveredTools(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{
		ID:     "coder-1",
		Type:   "coder",
		Status: "running",
		PID:    42,
		Config: agent.Config{
			Name:       "coder",
			Type:       "coder",
			MCPServers: []agent.MCPServer{{Name: "search", URL: "http://localhost:9001/mcp"}},
			Loop:       agent.Loop{Name: "react", MaxSteps: 1},
			Exec:       []string{"sleep", "1"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})
	app.mcpClientFor = func(string) *mcp.Client {
		return mcp.NewClientWithHTTPClient(stubMCPHTTP{
			respond: func(*http.Request) (*http.Response, error) {
				return mcpJSON(200, `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"web","description":"Search the web"},{"name":"code","description":"Search the code"}]}}`), nil
			},
		})
	}

	exitCode := app.Run(context.Background(), []string{"tool", "mcp", "ls", "coder-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	output := out.String()
	if !strings.Contains(output, "search\thttp\thttp://localhost:9001/mcp\tcode\t") {
		t.Fatalf("output missing code tool: %s", output)
	}
	if !strings.Contains(output, "search\thttp\thttp://localhost:9001/mcp\tweb\t") {
		t.Fatalf("output missing web tool: %s", output)
	}
}

func TestToolExecCallsMCPAndTraces(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	tracePath := filepath.Join(dir, "coder.trace")
	if err := repo.Save(store.Instance{
		ID:        "coder-1",
		Type:      "coder",
		Status:    "running",
		PID:       42,
		TracePath: tracePath,
		Config: agent.Config{
			Name:       "coder",
			Type:       "coder",
			MCPServers: []agent.MCPServer{{Name: "search", URL: "http://localhost:9001/mcp"}},
			Loop:       agent.Loop{Name: "react", MaxSteps: 1},
			Exec:       []string{"sleep", "1"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{pid: 42})
	app.now = func() time.Time { return now }
	app.mcpClientFor = func(string) *mcp.Client {
		return mcp.NewClientWithHTTPClient(stubMCPHTTP{
			respond: func(*http.Request) (*http.Response, error) {
				return mcpJSON(200, `{"jsonrpc":"2.0","id":1,"result":{"isError":false,"content":[{"type":"text","text":"hello"}]}}`), nil
			},
		})
	}

	exitCode := app.Run(context.Background(), []string{"exec", "--args", `{"q":"agents"}`, "coder-1", "search"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "hello") {
		t.Fatalf("output missing tool content: %s", out.String())
	}

	contents, err := readFileForTest(tracePath)
	if err != nil {
		t.Fatalf("read trace returned error: %v", err)
	}
	if !strings.Contains(contents, `"kind":"tool"`) {
		t.Fatalf("trace did not record tool event: %s", contents)
	}
	if !strings.Contains(contents, `"tool":"search"`) {
		t.Fatalf("trace did not record tool name: %s", contents)
	}
}

func TestToolExecRejectsMissingMCPServer(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{
		ID:        "coder-1",
		Type:      "coder",
		Status:    "running",
		Config:    agent.Config{Name: "coder", Type: "coder", Loop: agent.Loop{Name: "react", MaxSteps: 1}, Exec: []string{"sleep", "1"}},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{})

	exitCode := app.Run(context.Background(), []string{"exec", "coder-1", "search"})
	if exitCode == 0 {
		t.Fatal("exitCode = 0, want failure when no MCP servers configured")
	}
	if !strings.Contains(errOut.String(), "no MCP servers") {
		t.Fatalf("stderr missing no-MCP message: %s", errOut.String())
	}
}

func TestToolExecPropagatesMCPError(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	tracePath := filepath.Join(dir, "coder.trace")
	if err := repo.Save(store.Instance{
		ID:        "coder-1",
		Type:      "coder",
		TracePath: tracePath,
		Config: agent.Config{
			Name:       "coder",
			Type:       "coder",
			MCPServers: []agent.MCPServer{{Name: "search", URL: "http://localhost:9001/mcp"}},
			Loop:       agent.Loop{Name: "react", MaxSteps: 1},
			Exec:       []string{"sleep", "1"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{})
	app.now = func() time.Time { return now }
	app.mcpClientFor = func(string) *mcp.Client {
		return mcp.NewClientWithHTTPClient(stubMCPHTTP{
			respond: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			},
		})
	}

	exitCode := app.Run(context.Background(), []string{"exec", "coder-1", "search"})
	if exitCode == 0 {
		t.Fatal("exitCode = 0, want failure on MCP error")
	}

	contents, err := readFileForTest(tracePath)
	if err != nil {
		t.Fatalf("read trace returned error: %v", err)
	}
	if !strings.Contains(contents, `"status":"error"`) {
		t.Fatalf("trace missing error status: %s", contents)
	}
}

func readFileForTest(path string) (string, error) {
	body, err := readAllFromPath(path)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
