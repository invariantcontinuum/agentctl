// Package agentsdk is agentctl's provider-neutral agent runtime, bringing
// the patterns from the Anthropic Agent SDK, OpenAI Agents SDK, and Google
// ADK-Go into a single Go-stdlib-only package that lives behind small SOLID
// interfaces.
//
// The core composition is:
//
//	Runnable                    -- "run on input, return result" surface
//	  Agent                     -- single-model loop with tools + hooks + guards
//	  SequentialAgent           -- multi-agent in series
//	  ParallelAgent             -- multi-agent in parallel
//	  LoopAgent                 -- repeat one agent until predicate
//	  HandoffAgent              -- router agent picks one of N children
//
//	ModelClient                 -- provider-neutral generation interface
//	  AnthropicClient           -- Messages API
//	  OpenAIClient              -- Chat Completions API (works for vLLM, llama.cpp)
//	  GeminiClient              -- generateContent
//	  EchoClient                -- deterministic fallback for tests
//
//	Tool                        -- callable tool surface
//	  FunctionTool              -- wraps a Go func
//	  MCPTool                   -- wraps one MCP server tool
//
//	Session                     -- conversation history persistence
//	  MemorySession             -- in-process
//	  FileSession               -- JSON-Lines on disk
//
// All adapters convert between the package's neutral Message / ContentBlock
// shape and each provider's wire format. The Agent loop never touches HTTP
// or JSON itself.
package agentsdk

import "encoding/json"

// Role identifies who authored a Message.
type Role string

// Role constants. RoleSystem is conceptual: the agent's system prompt is sent
// once via GenerateRequest.System, not as a regular message. RoleTool covers
// tool result blocks regardless of how the underlying provider routes them.
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ContentBlock kinds. Use exactly one payload field per block.
const (
	BlockText       = "text"
	BlockToolUse    = "tool_use"
	BlockToolResult = "tool_result"
)

// ContentBlock is one element inside a Message. The package keeps every
// payload (text, tool call, tool result) in the same struct so the Agent
// loop can iterate without type switches at every layer.
type ContentBlock struct {
	Type string `json:"type"`

	// Text payload — used when Type == BlockText.
	Text string `json:"text,omitempty"`

	// Tool call payload — Type == BlockToolUse.
	ToolUseID string          `json:"tool_use_id,omitempty"`
	ToolName  string          `json:"tool_name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`

	// Tool result payload — Type == BlockToolResult.
	Output  string `json:"output,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

// TextBlock is the most common constructor.
func TextBlock(text string) ContentBlock {
	return ContentBlock{Type: BlockText, Text: text}
}

// ToolUseBlock declares a tool call request emitted by the model.
func ToolUseBlock(id, name string, input json.RawMessage) ContentBlock {
	return ContentBlock{Type: BlockToolUse, ToolUseID: id, ToolName: name, Input: input}
}

// ToolResultBlock carries the executor's output back to the model.
func ToolResultBlock(id, output string, isError bool) ContentBlock {
	return ContentBlock{Type: BlockToolResult, ToolUseID: id, Output: output, IsError: isError}
}

// Message is one entry in a conversation history.
type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// UserMessage is a one-block convenience constructor.
func UserMessage(text string) Message {
	return Message{Role: RoleUser, Content: []ContentBlock{TextBlock(text)}}
}

// AssistantMessage assembles an assistant turn.
func AssistantMessage(blocks ...ContentBlock) Message {
	return Message{Role: RoleAssistant, Content: blocks}
}

// ToolResultMessage wraps tool results into a single user-side message —
// every supported provider expects tool_result blocks attached to a user
// turn, not an assistant turn.
func ToolResultMessage(results ...ContentBlock) Message {
	return Message{Role: RoleUser, Content: results}
}

// FirstText returns the first text block in a message, or "" if none.
func (m Message) FirstText() string {
	for _, block := range m.Content {
		if block.Type == BlockText {
			return block.Text
		}
	}
	return ""
}

// ToolUses returns every tool_use block in a message.
func (m Message) ToolUses() []ContentBlock {
	out := make([]ContentBlock, 0, len(m.Content))
	for _, block := range m.Content {
		if block.Type == BlockToolUse {
			out = append(out, block)
		}
	}
	return out
}
