package agentsdk

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/mcp"
)

type fakeMCP struct {
	tools    map[string][]mcp.Tool
	output   map[string]mcp.CallResult
	listErr  map[string]error
	callArgs map[string]map[string]any
}

func (f *fakeMCP) ListTools(_ context.Context, server mcp.ServerSpec) ([]mcp.Tool, error) {
	if f.listErr != nil {
		if err, ok := f.listErr[server.Name]; ok {
			return nil, err
		}
	}
	return f.tools[server.Name], nil
}

func (f *fakeMCP) Call(_ context.Context, server mcp.ServerSpec, name string, arguments map[string]any) (mcp.CallResult, error) {
	if f.callArgs == nil {
		f.callArgs = map[string]map[string]any{}
	}
	f.callArgs[server.Name+":"+name] = arguments
	if hit, ok := f.output[server.Name+":"+name]; ok {
		return hit, nil
	}
	return mcp.CallResult{Content: json.RawMessage(`"ok"`)}, nil
}

func TestDiscoverMCPToolsWrapsServerTools(t *testing.T) {
	servers := []agent.MCPServer{
		{Name: "search", URL: "http://example/mcp"},
	}
	client := &fakeMCP{
		tools: map[string][]mcp.Tool{
			"search": {{Name: "lookup", Description: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}},
		},
	}
	tools, err := DiscoverMCPTools(context.Background(), client, servers, nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(tools))
	}
	if tools[0].Name() != "lookup" {
		t.Fatalf("name = %q", tools[0].Name())
	}
	if !strings.Contains(string(tools[0].InputSchema()), `"type":"object"`) {
		t.Fatalf("schema lost: %s", tools[0].InputSchema())
	}
}

func TestDiscoverMCPToolsReportsServerErrorAndContinues(t *testing.T) {
	servers := []agent.MCPServer{
		{Name: "broken", URL: "http://broken"},
		{Name: "ok", URL: "http://ok"},
	}
	client := &fakeMCP{
		tools: map[string][]mcp.Tool{
			"ok": {{Name: "ping"}},
		},
		listErr: map[string]error{"broken": errors.New("offline")},
	}
	var collected []string
	tools, err := DiscoverMCPTools(context.Background(), client, servers, func(server agent.MCPServer, err error) {
		collected = append(collected, server.Name+":"+err.Error())
	})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(tools) != 1 || tools[0].Name() != "ping" {
		t.Fatalf("partial discovery failed: %+v", tools)
	}
	if len(collected) != 1 || !strings.Contains(collected[0], "broken") {
		t.Fatalf("error callback miss: %v", collected)
	}
}

func TestMCPToolExecuteForwardsArguments(t *testing.T) {
	client := &fakeMCP{
		tools: map[string][]mcp.Tool{
			"sv": {{Name: "fn"}},
		},
		output: map[string]mcp.CallResult{
			"sv:fn": {Content: json.RawMessage(`"answer"`)},
		},
	}
	tools, err := DiscoverMCPTools(context.Background(), client, []agent.MCPServer{
		{Name: "sv", URL: "http://x"},
	}, nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	out, err := tools[0].Execute(context.Background(), json.RawMessage(`{"q":"hi"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != `"answer"` {
		t.Fatalf("output = %q", out)
	}
	if got := client.callArgs["sv:fn"]["q"]; got != "hi" {
		t.Fatalf("args = %+v", client.callArgs["sv:fn"])
	}
}

func TestMCPToolExecuteWrapsNonObjectInput(t *testing.T) {
	client := &fakeMCP{tools: map[string][]mcp.Tool{"sv": {{Name: "fn"}}}}
	tools, _ := DiscoverMCPTools(context.Background(), client, []agent.MCPServer{
		{Name: "sv", URL: "http://x"},
	}, nil)
	if _, err := tools[0].Execute(context.Background(), json.RawMessage(`"hello"`)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, ok := client.callArgs["sv:fn"]["input"]; !ok || got != "hello" {
		t.Fatalf("non-object input not wrapped: %+v", client.callArgs["sv:fn"])
	}
}
