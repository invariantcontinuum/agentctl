package agentsdk

import "context"

// StopReason values returned by ModelClient.Generate. The Agent uses these
// to decide whether to break the loop, dispatch tools, or surface an error.
const (
	StopReasonEndTurn   = "end_turn"
	StopReasonToolUse   = "tool_use"
	StopReasonMaxTokens = "max_tokens"
	StopReasonError     = "error"
)

// ModelClient is the provider-neutral generation surface. Implementations
// (Anthropic, OpenAI, Gemini, Echo) translate to and from each provider's
// wire format. Streaming is intentionally not in this v1 — both Anthropic
// and OpenAI shape streaming differently enough that a unified API would
// hide useful details from agentctl users.
type ModelClient interface {
	Provider() string
	Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error)
}

// GenerateRequest is what the Agent hands to the model on every step.
type GenerateRequest struct {
	Model     string
	System    string
	Messages  []Message
	Tools     []ToolSpec
	MaxTokens int
}

// GenerateResponse is the assistant turn returned by the model. Content is
// the new assistant blocks (text and/or tool_use) — the Agent appends it
// directly to the session.
type GenerateResponse struct {
	Provider   string
	Content    []ContentBlock
	StopReason string
}
