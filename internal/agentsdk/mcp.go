package agentsdk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/mcp"
)

// MCPClient is the surface MCPTool needs from the mcp package. The full
// concrete *mcp.Client satisfies it; tests can inject a fake.
type MCPClient interface {
	ListTools(ctx context.Context, server mcp.ServerSpec) ([]mcp.Tool, error)
	Call(ctx context.Context, server mcp.ServerSpec, name string, arguments map[string]any) (mcp.CallResult, error)
}

// MCPTool wraps one tool exposed by an MCP server so the Agent can call it
// like any other Tool. Discovery happens once via DiscoverMCPTools and is
// cached for the lifetime of the run; the model decides which to invoke.
type MCPTool struct {
	server      mcp.ServerSpec
	name        string
	description string
	schema      json.RawMessage
	client      MCPClient
}

// Name implements Tool.
func (t *MCPTool) Name() string { return t.name }

// Description implements Tool.
func (t *MCPTool) Description() string { return t.description }

// InputSchema implements Tool.
func (t *MCPTool) InputSchema() json.RawMessage {
	if len(t.schema) == 0 {
		return json.RawMessage(`{"type":"object"}`)
	}
	return t.schema
}

// Execute implements Tool. The argument JSON is decoded into a map[string]any
// so it can be re-encoded by the MCP client. A non-object input is wrapped
// under {"input": <raw>} so a tool that expects a single positional value
// still receives it.
func (t *MCPTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	arguments, err := decodeMCPInput(input)
	if err != nil {
		return "", fmt.Errorf("mcp tool %q: %w", t.name, err)
	}
	result, err := t.client.Call(ctx, t.server, t.name, arguments)
	if err != nil {
		return "", fmt.Errorf("mcp tool %q: %w", t.name, err)
	}
	if result.IsError {
		return string(result.Content), fmt.Errorf("mcp tool %q reported an error", t.name)
	}
	if len(result.Content) == 0 {
		return "", nil
	}
	return string(result.Content), nil
}

// DiscoverMCPTools lists every tool on each server and returns Tool wrappers.
// A failing server is logged via the optional onServerError callback rather
// than aborting discovery — agentctl prefers a partial tool set over zero.
func DiscoverMCPTools(ctx context.Context, client MCPClient, servers []agent.MCPServer, onServerError func(server agent.MCPServer, err error)) ([]Tool, error) {
	if client == nil {
		return nil, nil
	}
	out := make([]Tool, 0)
	for _, server := range servers {
		spec := mcpServerSpecOf(server)
		tools, err := client.ListTools(ctx, spec)
		if err != nil {
			if onServerError != nil {
				onServerError(server, err)
			}
			continue
		}
		for _, tool := range tools {
			out = append(out, &MCPTool{
				server:      spec,
				name:        tool.Name,
				description: tool.Description,
				schema:      tool.InputSchema,
				client:      client,
			})
		}
	}
	return out, nil
}

func mcpServerSpecOf(server agent.MCPServer) mcp.ServerSpec {
	transport := mcp.TransportHTTP
	if server.Command != "" {
		transport = mcp.TransportStdio
	}
	return mcp.ServerSpec{
		Name:      server.Name,
		Transport: transport,
		URL:       server.URL,
		BasePath:  server.BasePath,
		Headers:   copyStringMap(server.Headers),
		Command:   server.Command,
		Args:      append([]string{}, server.Args...),
		Env:       copyStringMap(server.Env),
	}
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

// decodeMCPInput accepts either a JSON object (passed through as a map) or
// any other JSON value (wrapped under "input"). Empty input maps to an empty
// object so the wire shape stays consistent.
func decodeMCPInput(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err == nil {
		return asMap, nil
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return map[string]any{"input": value}, nil
}
