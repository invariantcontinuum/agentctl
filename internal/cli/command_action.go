package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/mcp"
	"github.com/invariantcontinuum/agentctl/internal/trace"
)

func (a *App) toolMCP(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "ls" {
		return fmt.Errorf("usage: agentctl tool mcp ls <agent-id>")
	}
	id, err := requiredID("tool mcp ls", args[1:])
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	if len(instance.Config.MCPServers) == 0 {
		return nil
	}

	client := a.mcpClientFor(id)
	for _, server := range instance.Config.MCPServers {
		spec := toMCPServerSpec(server)
		address := mcpServerSummary(server)
		tools, err := client.ListTools(ctx, spec)
		if err != nil {
			fmt.Fprintf(a.out, "%s\t%s\t%s\tERROR\t%v\n", server.Name, server.Transport, address, err)
			continue
		}
		if len(tools) == 0 {
			fmt.Fprintf(a.out, "%s\t%s\t%s\t-\n", server.Name, server.Transport, address)
			continue
		}
		for _, tool := range tools {
			fmt.Fprintf(a.out, "%s\t%s\t%s\t%s\t%s\n", server.Name, server.Transport, address, tool.Name, summarize(tool.Description))
		}
	}
	return nil
}

func toMCPServerSpec(server agent.MCPServer) mcp.ServerSpec {
	return mcp.ServerSpec{
		Name:      server.Name,
		Transport: server.Transport,
		URL:       server.URL,
		Command:   server.Command,
		Args:      server.Args,
		Env:       server.Env,
	}
}

func (a *App) toolExec(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("tool exec", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	server := flags.String("server", "", "MCP server name (defaults to first)")
	jsonArgs := flags.String("args", "", "JSON-encoded arguments object")
	if err := flags.Parse(args); err != nil {
		return err
	}
	rest := flags.Args()
	if len(rest) != 2 {
		return fmt.Errorf("usage: agentctl tool exec [--server NAME] [--args JSON] <agent-id> <tool>")
	}
	id := rest[0]
	toolName := rest[1]

	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	target, err := selectMCPServer(instance.Config.MCPServers, *server)
	if err != nil {
		return err
	}

	arguments, err := parseToolArguments(*jsonArgs)
	if err != nil {
		return err
	}

	start := a.now()
	client := a.mcpClientFor(id)
	result, err := client.Call(ctx, toMCPServerSpec(target), toolName, arguments)
	latency := a.now().Sub(start)

	traceEvent := trace.Event{
		Time:   a.now().UTC(),
		Kind:   trace.KindTool,
		Agent:  id,
		Detail: fmt.Sprintf("tool=%s server=%s", toolName, target.Name),
		Fields: map[string]string{
			"server":     target.Name,
			"tool":       toolName,
			"latency_ms": fmt.Sprintf("%d", latency.Milliseconds()),
			"status":     statusOf(result.IsError, err),
		},
	}
	if traceErr := a.writeTrace(instance.TracePath, traceEvent); traceErr != nil {
		return traceErr
	}
	if err != nil {
		return err
	}

	if len(result.Content) == 0 {
		return nil
	}
	pretty, err := json.MarshalIndent(json.RawMessage(result.Content), "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, string(pretty))
	return nil
}

// exec is a top-level alias for `tool exec`.
//
//	agentctl exec [--server NAME] [--args JSON] <agent-id> <tool>
//
// Flags must come before the positional <agent-id> and <tool>, matching Go's
// flag package behavior and `docker exec [OPTIONS] CONTAINER COMMAND`.
func (a *App) exec(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: agentctl exec [--server NAME] [--args JSON] <agent-id> <tool>")
	}
	return a.toolExec(ctx, args)
}

func selectMCPServer(servers []agent.MCPServer, name string) (agent.MCPServer, error) {
	if len(servers) == 0 {
		return agent.MCPServer{}, fmt.Errorf("agent has no MCP servers configured")
	}
	if name == "" {
		return servers[0], nil
	}
	for _, server := range servers {
		if server.Name == name {
			return server, nil
		}
	}
	return agent.MCPServer{}, fmt.Errorf("agent has no MCP server named %q", name)
}

func parseToolArguments(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var arguments map[string]any
	if err := json.Unmarshal([]byte(raw), &arguments); err != nil {
		return nil, fmt.Errorf("--args must be a JSON object: %w", err)
	}
	return arguments, nil
}

func summarize(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return "-"
	}
	if len(description) > 80 {
		return description[:77] + "..."
	}
	return description
}

func statusOf(isError bool, err error) string {
	if err != nil {
		return "error"
	}
	if isError {
		return "tool_error"
	}
	return "ok"
}

// Ensure time package is referenced when this file is compiled standalone.
var _ = time.Time{}
