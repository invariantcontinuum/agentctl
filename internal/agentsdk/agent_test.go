package agentsdk

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

// scriptedModel returns each scripted GenerateResponse in order; once the
// script is exhausted it errors so the loop's MaxSteps cap is never hit
// silently.
type scriptedModel struct {
	provider  string
	responses []GenerateResponse
	calls     atomic.Int32
}

func (s *scriptedModel) Provider() string { return s.provider }

func (s *scriptedModel) Generate(_ context.Context, _ GenerateRequest) (GenerateResponse, error) {
	index := int(s.calls.Add(1)) - 1
	if index >= len(s.responses) {
		return GenerateResponse{}, errors.New("script exhausted")
	}
	return s.responses[index], nil
}

func TestAgentTextOnlyTerminatesAfterOneStep(t *testing.T) {
	model := &scriptedModel{provider: "scripted", responses: []GenerateResponse{
		{Content: []ContentBlock{TextBlock("hello back")}, StopReason: StopReasonEndTurn},
	}}
	agent := NewAgent("greeter", model)

	result, err := agent.Run(context.Background(), nil, "hi")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Final != "hello back" {
		t.Fatalf("final = %q", result.Final)
	}
	if result.Steps != 1 {
		t.Fatalf("steps = %d", result.Steps)
	}
}

func TestAgentDispatchesToolAndContinues(t *testing.T) {
	model := &scriptedModel{provider: "scripted", responses: []GenerateResponse{
		{
			Content:    []ContentBlock{ToolUseBlock("call-1", "echo", json.RawMessage(`{"value":"abc"}`))},
			StopReason: StopReasonToolUse,
		},
		{
			Content:    []ContentBlock{TextBlock("done")},
			StopReason: StopReasonEndTurn,
		},
	}}
	agent := NewAgent("tool-user", model)
	agent.Tools.Register(NewFunctionTool("echo", "echoes input", nil, func(_ context.Context, input json.RawMessage) (string, error) {
		return string(input), nil
	}))

	result, err := agent.Run(context.Background(), nil, "use tool")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Final != "done" {
		t.Fatalf("final = %q", result.Final)
	}
	if model.calls.Load() != 2 {
		t.Fatalf("model calls = %d", model.calls.Load())
	}
}

func TestAgentReportsMissingTool(t *testing.T) {
	model := &scriptedModel{provider: "scripted", responses: []GenerateResponse{
		{
			Content:    []ContentBlock{ToolUseBlock("call-1", "absent", json.RawMessage(`{}`))},
			StopReason: StopReasonToolUse,
		},
		{
			Content:    []ContentBlock{TextBlock("recovered")},
			StopReason: StopReasonEndTurn,
		},
	}}
	agent := NewAgent("ghost", model)
	if _, err := agent.Run(context.Background(), nil, "go"); err != nil {
		t.Fatalf("run: %v", err)
	}
	// Inspect the session by re-running with a captured session.
	session := NewMemorySession("ghost")
	model.calls.Store(0)
	if _, err := agent.Run(context.Background(), session, "go"); err != nil {
		t.Fatalf("run: %v", err)
	}
	transcript := session.Messages()
	if len(transcript) < 3 {
		t.Fatalf("transcript = %d messages", len(transcript))
	}
	results := transcript[2]
	if results.Role != RoleUser || len(results.Content) == 0 || !results.Content[0].IsError {
		t.Fatalf("missing tool not reported as error: %+v", results)
	}
}

func TestAgentGuardrailAborts(t *testing.T) {
	model := &scriptedModel{provider: "scripted", responses: []GenerateResponse{
		{Content: []ContentBlock{TextBlock("forbidden token")}, StopReason: StopReasonEndTurn},
	}}
	agent := NewAgent("guarded", model)
	agent.Guards = []Guardrail{&MaxLengthGuard{GuardName: "max-len", Max: 5}}

	_, err := agent.Run(context.Background(), nil, "hi")
	if err == nil {
		t.Fatalf("expected guard to abort")
	}
	if !strings.Contains(err.Error(), "max-len") {
		t.Fatalf("err = %v", err)
	}
}

func TestAgentHooksFire(t *testing.T) {
	model := &scriptedModel{provider: "scripted", responses: []GenerateResponse{
		{Content: []ContentBlock{TextBlock("yo")}, StopReason: StopReasonEndTurn},
	}}
	agent := NewAgent("hooked", model)
	var beforeRunCount, afterRunCount int
	agent.Hooks.BeforeRun = func(_ context.Context, _ *Agent, _ string) { beforeRunCount++ }
	agent.Hooks.AfterRun = func(_ context.Context, _ *Agent, _ RunResult, _ error) { afterRunCount++ }

	if _, err := agent.Run(context.Background(), nil, "hi"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if beforeRunCount != 1 || afterRunCount != 1 {
		t.Fatalf("hooks fired wrong: before=%d after=%d", beforeRunCount, afterRunCount)
	}
}

func TestAgentHonorsContextCancel(t *testing.T) {
	model := &scriptedModel{provider: "scripted", responses: []GenerateResponse{
		{Content: []ContentBlock{TextBlock("never")}, StopReason: StopReasonEndTurn},
	}}
	agent := NewAgent("ctx", model)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := agent.Run(ctx, nil, "hi")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
