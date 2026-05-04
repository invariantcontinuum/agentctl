package agentsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAIClient targets any /v1/chat/completions endpoint that follows the
// OpenAI Chat Completions wire format. vLLM and llama.cpp's OpenAI-compat
// servers both work — pass them as endpoint with apiKey="" since they
// usually don't require authentication.
type OpenAIClient struct {
	provider string
	endpoint string
	apiKey   string
	model    string
	client   HTTPClient
}

// NewOpenAIClient constructs an OpenAI-compatible client. provider is the
// label surfaced via Provider(); use "openai", "vllm", "llamacpp", etc.
func NewOpenAIClient(provider string, endpoint string, apiKey string, model string, http HTTPClient) *OpenAIClient {
	if provider == "" {
		provider = "openai"
	}
	if http == nil {
		http = defaultHTTPClient()
	}
	return &OpenAIClient{
		provider: provider,
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		client:   http,
	}
}

// Provider implements ModelClient.
func (c *OpenAIClient) Provider() string { return c.provider }

// Generate implements ModelClient.
func (c *OpenAIClient) Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error) {
	if c.endpoint == "" {
		return GenerateResponse{}, errors.New("openai: endpoint is empty")
	}
	model := request.Model
	if model == "" {
		model = c.model
	}
	if model == "" {
		return GenerateResponse{}, errors.New("openai: model name is empty")
	}

	body := openAIRequest{
		Model:    model,
		Messages: encodeOpenAIMessages(request.System, request.Messages),
		Tools:    encodeOpenAITools(request.Tools),
		Stream:   false,
	}
	if request.MaxTokens > 0 {
		body.MaxTokens = request.MaxTokens
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return GenerateResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return GenerateResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return GenerateResponse{}, err
	}
	defer httpResp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return GenerateResponse{}, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return GenerateResponse{}, &HTTPError{
			Provider:   c.provider,
			StatusCode: httpResp.StatusCode,
			Body:       strings.TrimSpace(string(raw)),
		}
	}

	var decoded openAIResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return GenerateResponse{}, fmt.Errorf("openai decode: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return GenerateResponse{}, errors.New("openai: empty choices")
	}
	choice := decoded.Choices[0]
	return GenerateResponse{
		Provider:   c.provider,
		Content:    decodeOpenAIChoice(choice),
		StopReason: mapOpenAIStop(choice.FinishReason),
	}, nil
}

// --- wire shapes -----------------------------------------------------------

type openAIRequest struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	Tools     []openAITool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIToolCallFunc `json:"function"`
}

type openAIToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAITool struct {
	Type     string                 `json:"type"`
	Function openAIToolFunctionSpec `json:"function"`
}

type openAIToolFunctionSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

func encodeOpenAIMessages(system string, messages []Message) []openAIMessage {
	out := make([]openAIMessage, 0, len(messages)+1)
	if system != "" {
		out = append(out, openAIMessage{Role: "system", Content: system})
	}
	for _, message := range messages {
		switch message.Role {
		case RoleAssistant:
			out = append(out, encodeOpenAIAssistant(message))
		default:
			out = append(out, encodeOpenAIUserOrTool(message)...)
		}
	}
	return out
}

func encodeOpenAIAssistant(message Message) openAIMessage {
	out := openAIMessage{Role: "assistant"}
	textParts := make([]string, 0, len(message.Content))
	calls := make([]openAIToolCall, 0)
	for _, block := range message.Content {
		switch block.Type {
		case BlockText:
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case BlockToolUse:
			calls = append(calls, openAIToolCall{
				ID:   block.ToolUseID,
				Type: "function",
				Function: openAIToolCallFunc{
					Name:      block.ToolName,
					Arguments: rawToString(block.Input),
				},
			})
		}
	}
	if len(textParts) > 0 {
		out.Content = strings.Join(textParts, "\n")
	}
	if len(calls) > 0 {
		out.ToolCalls = calls
	}
	return out
}

// encodeOpenAIUserOrTool turns one user message into one or many openAI
// messages: text blocks become a user role; tool_result blocks become tool
// role messages, one per result so each carries its own tool_call_id.
func encodeOpenAIUserOrTool(message Message) []openAIMessage {
	out := make([]openAIMessage, 0, len(message.Content))
	textParts := make([]string, 0, len(message.Content))
	for _, block := range message.Content {
		switch block.Type {
		case BlockText:
			textParts = append(textParts, block.Text)
		case BlockToolResult:
			out = append(out, openAIMessage{
				Role:       "tool",
				ToolCallID: block.ToolUseID,
				Content:    block.Output,
			})
		}
	}
	if len(textParts) > 0 {
		out = append([]openAIMessage{{Role: "user", Content: strings.Join(textParts, "\n")}}, out...)
	}
	return out
}

func encodeOpenAITools(tools []ToolSpec) []openAITool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]openAITool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, openAITool{
			Type: "function",
			Function: openAIToolFunctionSpec{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  defaultJSONObject(tool.InputSchema),
			},
		})
	}
	return out
}

func decodeOpenAIChoice(choice openAIChoice) []ContentBlock {
	out := make([]ContentBlock, 0, 1+len(choice.Message.ToolCalls))
	if text := choice.Message.Content; text != "" {
		out = append(out, TextBlock(text))
	}
	for _, call := range choice.Message.ToolCalls {
		out = append(out, ToolUseBlock(call.ID, call.Function.Name, json.RawMessage(call.Function.Arguments)))
	}
	return out
}

func mapOpenAIStop(reason string) string {
	switch reason {
	case "stop":
		return StopReasonEndTurn
	case "tool_calls":
		return StopReasonToolUse
	case "length":
		return StopReasonMaxTokens
	default:
		return reason
	}
}

func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}
