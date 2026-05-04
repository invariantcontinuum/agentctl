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

// HTTPClient is the surface every HTTP-backed ModelClient implementation
// needs. Tests inject a stub so wire-format checks don't open sockets.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// AnthropicClient targets the Messages API on api.anthropic.com (or any
// gateway that speaks the same wire format).
type AnthropicClient struct {
	endpoint string
	apiKey   string
	model    string
	client   HTTPClient
	version  string
}

// NewAnthropicClient constructs an Anthropic client. apiKey is required; an
// empty value returns a client that errors on every Generate call so a
// misconfigured provider fails loudly instead of silently echoing.
func NewAnthropicClient(endpoint string, apiKey string, model string, http HTTPClient) *AnthropicClient {
	if http == nil {
		http = defaultHTTPClient()
	}
	return &AnthropicClient{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		client:   http,
		version:  "2023-06-01",
	}
}

// Provider implements ModelClient.
func (c *AnthropicClient) Provider() string { return "anthropic" }

// Generate implements ModelClient.
func (c *AnthropicClient) Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error) {
	if c.endpoint == "" {
		return GenerateResponse{}, errors.New("anthropic: endpoint is empty")
	}
	if c.apiKey == "" {
		return GenerateResponse{}, errors.New("anthropic: api key is empty")
	}

	model := request.Model
	if model == "" {
		model = c.model
	}
	if model == "" {
		return GenerateResponse{}, errors.New("anthropic: model name is empty")
	}

	body := anthropicRequest{
		Model:     model,
		MaxTokens: maxTokensOrDefault(request.MaxTokens, 1024),
		System:    request.System,
		Messages:  encodeAnthropicMessages(request.Messages),
		Tools:     encodeAnthropicTools(request.Tools),
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return GenerateResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/v1/messages", bytes.NewReader(encoded))
	if err != nil {
		return GenerateResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", c.version)

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
			Provider:   "anthropic",
			StatusCode: httpResp.StatusCode,
			Body:       strings.TrimSpace(string(raw)),
		}
	}

	var decoded anthropicResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return GenerateResponse{}, fmt.Errorf("anthropic decode: %w", err)
	}
	return GenerateResponse{
		Provider:   "anthropic",
		Content:    decodeAnthropicContent(decoded.Content),
		StopReason: mapAnthropicStop(decoded.StopReason),
	}, nil
}

// --- wire shapes -----------------------------------------------------------

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicResponse struct {
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
}

func encodeAnthropicMessages(messages []Message) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(messages))
	for _, message := range messages {
		role := "user"
		if message.Role == RoleAssistant {
			role = "assistant"
		}
		blocks := make([]anthropicContent, 0, len(message.Content))
		for _, block := range message.Content {
			switch block.Type {
			case BlockText:
				blocks = append(blocks, anthropicContent{Type: "text", Text: block.Text})
			case BlockToolUse:
				blocks = append(blocks, anthropicContent{
					Type:  "tool_use",
					ID:    block.ToolUseID,
					Name:  block.ToolName,
					Input: defaultJSONObject(block.Input),
				})
			case BlockToolResult:
				blocks = append(blocks, anthropicContent{
					Type:      "tool_result",
					ToolUseID: block.ToolUseID,
					Content:   block.Output,
					IsError:   block.IsError,
				})
			}
		}
		out = append(out, anthropicMessage{Role: role, Content: blocks})
	}
	return out
}

func encodeAnthropicTools(tools []ToolSpec) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropicTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: defaultJSONObject(tool.InputSchema),
		})
	}
	return out
}

func decodeAnthropicContent(content []anthropicContent) []ContentBlock {
	out := make([]ContentBlock, 0, len(content))
	for _, block := range content {
		switch block.Type {
		case "text":
			out = append(out, TextBlock(block.Text))
		case "tool_use":
			out = append(out, ToolUseBlock(block.ID, block.Name, defaultJSONObject(block.Input)))
		}
	}
	return out
}

func mapAnthropicStop(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return StopReasonEndTurn
	case "tool_use":
		return StopReasonToolUse
	case "max_tokens":
		return StopReasonMaxTokens
	default:
		return reason
	}
}
