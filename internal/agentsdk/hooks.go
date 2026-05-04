package agentsdk

import (
	"context"
	"encoding/json"
)

// Hooks fire at fixed points in Agent.Run. Each callback is optional;
// callers leave fields nil for the points they don't care about. The Agent
// uses these to mirror the Anthropic Agent SDK's "hooks", the OpenAI Agents
// SDK's "lifecycle", and Google ADK-Go's "callbacks" — same idea, three
// SDKs, one struct here.
type Hooks struct {
	BeforeRun  func(ctx context.Context, agent *Agent, input string)
	AfterRun   func(ctx context.Context, agent *Agent, result RunResult, err error)
	BeforeTool func(ctx context.Context, tool Tool, input json.RawMessage)
	AfterTool  func(ctx context.Context, tool Tool, output string, err error)
}

func (h Hooks) fireBeforeRun(ctx context.Context, agent *Agent, input string) {
	if h.BeforeRun != nil {
		h.BeforeRun(ctx, agent, input)
	}
}

func (h Hooks) fireAfterRun(ctx context.Context, agent *Agent, result RunResult, err error) {
	if h.AfterRun != nil {
		h.AfterRun(ctx, agent, result, err)
	}
}

func (h Hooks) fireBeforeTool(ctx context.Context, tool Tool, input json.RawMessage) {
	if h.BeforeTool != nil {
		h.BeforeTool(ctx, tool, input)
	}
}

func (h Hooks) fireAfterTool(ctx context.Context, tool Tool, output string, err error) {
	if h.AfterTool != nil {
		h.AfterTool(ctx, tool, output, err)
	}
}
