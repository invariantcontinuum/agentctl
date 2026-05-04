package agentsdk

import (
	"context"
	"errors"
	"fmt"
)

// Runnable is the smallest "run on input" surface every agent type
// (single-model Agent, SequentialAgent, ParallelAgent, LoopAgent,
// HandoffAgent) implements. It is the glue that lets multi-agent
// orchestrators compose any combination of leaves and other orchestrators.
type Runnable interface {
	Name() string
	Run(ctx context.Context, session Session, input string) (RunResult, error)
}

// RunResult is what every Runnable returns. Final is the last assistant
// text the agent produced; Steps counts model calls; Messages is the full
// transcript including the user input that was just appended.
type RunResult struct {
	AgentName  string      `json:"agent"`
	Final      string      `json:"final"`
	Steps      int         `json:"steps"`
	Messages   []Message   `json:"messages,omitempty"`
	StopReason string      `json:"stop_reason,omitempty"`
	Children   []RunResult `json:"children,omitempty"`
}

// Agent is one model + tools loop. Construct it with NewAgent; the zero
// value is not usable.
type Agent struct {
	AgentName string
	Model     ModelClient
	ModelName string
	System    string
	Tools     *Registry
	Hooks     Hooks
	Guards    []Guardrail
	MaxSteps  int
	MaxTokens int
}

// NewAgent constructs an Agent. The Model is the only mandatory field; the
// rest have safe defaults so a smoke-test agent is one line.
func NewAgent(name string, model ModelClient) *Agent {
	if name == "" {
		name = "agent"
	}
	return &Agent{
		AgentName: name,
		Model:     model,
		Tools:     NewRegistry(),
		MaxSteps:  16,
		MaxTokens: 1024,
	}
}

// Name implements Runnable.
func (a *Agent) Name() string { return a.AgentName }

// Run implements Runnable. The loop is:
//  1. Append the user input to the session.
//  2. Send (system, messages, tools) to the model.
//  3. Append the assistant response to the session.
//  4. If the model emitted tool_use blocks, dispatch each one through the
//     Registry and append the results as a single user-side message.
//  5. Repeat until the stop reason is end_turn or MaxSteps is reached.
//
// Hooks (BeforeRun, AfterRun, BeforeTool, AfterTool) fire at the obvious
// points; Guards run on every assistant text emission.
func (a *Agent) Run(ctx context.Context, session Session, input string) (RunResult, error) {
	if a == nil || a.Model == nil {
		return RunResult{}, errors.New("agent: model is nil")
	}
	if session == nil {
		session = NewMemorySession("")
	}

	a.Hooks.fireBeforeRun(ctx, a, input)

	if err := session.Append(UserMessage(input)); err != nil {
		return RunResult{}, fmt.Errorf("session append user: %w", err)
	}

	result := RunResult{AgentName: a.AgentName}

	for step := 0; step < a.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			result.Steps = step
			a.Hooks.fireAfterRun(ctx, a, result, ctx.Err())
			return result, ctx.Err()
		default:
		}

		response, err := a.Model.Generate(ctx, GenerateRequest{
			Model:     a.ModelName,
			System:    a.System,
			Messages:  session.Messages(),
			Tools:     a.Tools.Specs(),
			MaxTokens: a.MaxTokens,
		})
		if err != nil {
			a.Hooks.fireAfterRun(ctx, a, result, err)
			return result, fmt.Errorf("model generate: %w", err)
		}

		assistant := Message{Role: RoleAssistant, Content: response.Content}
		if err := session.Append(assistant); err != nil {
			return result, fmt.Errorf("session append assistant: %w", err)
		}

		text := assistant.FirstText()
		if text != "" {
			if err := a.runGuards(ctx, text); err != nil {
				a.Hooks.fireAfterRun(ctx, a, result, err)
				return result, err
			}
			result.Final = text
		}

		toolCalls := assistant.ToolUses()
		if len(toolCalls) == 0 {
			result.Steps = step + 1
			result.StopReason = response.StopReason
			result.Messages = session.Messages()
			a.Hooks.fireAfterRun(ctx, a, result, nil)
			return result, nil
		}

		toolResults := make([]ContentBlock, 0, len(toolCalls))
		for _, call := range toolCalls {
			toolResults = append(toolResults, a.dispatchTool(ctx, call))
		}
		if err := session.Append(ToolResultMessage(toolResults...)); err != nil {
			return result, fmt.Errorf("session append tool results: %w", err)
		}
	}

	result.StopReason = "max_steps"
	result.Messages = session.Messages()
	a.Hooks.fireAfterRun(ctx, a, result, nil)
	return result, nil
}

func (a *Agent) dispatchTool(ctx context.Context, call ContentBlock) ContentBlock {
	tool, ok := a.Tools.Get(call.ToolName)
	if !ok {
		return ToolResultBlock(call.ToolUseID, fmt.Sprintf("tool %q not found", call.ToolName), true)
	}
	a.Hooks.fireBeforeTool(ctx, tool, call.Input)
	output, err := tool.Execute(ctx, call.Input)
	a.Hooks.fireAfterTool(ctx, tool, output, err)
	if err != nil {
		return ToolResultBlock(call.ToolUseID, err.Error(), true)
	}
	return ToolResultBlock(call.ToolUseID, output, false)
}

func (a *Agent) runGuards(ctx context.Context, text string) error {
	for _, guard := range a.Guards {
		if guard == nil {
			continue
		}
		if err := guard.Check(ctx, text); err != nil {
			return fmt.Errorf("guardrail %s: %w", guard.Name(), err)
		}
	}
	return nil
}
