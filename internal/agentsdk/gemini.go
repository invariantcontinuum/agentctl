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

// GeminiClient targets generativelanguage.googleapis.com's
// generateContent endpoint. ADK-Go consumes this same surface internally;
// we wire it directly so agentctl doesn't pull in the Google API SDKs.
type GeminiClient struct {
	endpoint string
	apiKey   string
	model    string
	client   HTTPClient
}

// NewGeminiClient constructs a Gemini client. apiKey is required (Gemini
// also supports OAuth, but agentctl persists API keys for now).
func NewGeminiClient(endpoint string, apiKey string, model string, http HTTPClient) *GeminiClient {
	if http == nil {
		http = defaultHTTPClient()
	}
	return &GeminiClient{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		client:   http,
	}
}

// Provider implements ModelClient.
func (c *GeminiClient) Provider() string { return "gemini" }

// Generate implements ModelClient.
func (c *GeminiClient) Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error) {
	if c.endpoint == "" {
		return GenerateResponse{}, errors.New("gemini: endpoint is empty")
	}
	if c.apiKey == "" {
		return GenerateResponse{}, errors.New("gemini: api key is empty")
	}
	model := request.Model
	if model == "" {
		model = c.model
	}
	if model == "" {
		return GenerateResponse{}, errors.New("gemini: model name is empty")
	}

	body := geminiRequest{
		Contents: encodeGeminiContents(request.Messages),
		Tools:    encodeGeminiTools(request.Tools),
	}
	if request.System != "" {
		body.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: request.System}},
		}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return GenerateResponse{}, err
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.endpoint, model, c.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return GenerateResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
			Provider:   "gemini",
			StatusCode: httpResp.StatusCode,
			Body:       strings.TrimSpace(string(raw)),
		}
	}

	var decoded geminiResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return GenerateResponse{}, fmt.Errorf("gemini decode: %w", err)
	}
	if len(decoded.Candidates) == 0 {
		return GenerateResponse{}, errors.New("gemini: empty candidates")
	}
	candidate := decoded.Candidates[0]
	return GenerateResponse{
		Provider:   "gemini",
		Content:    decodeGeminiParts(candidate.Content.Parts),
		StopReason: mapGeminiStop(candidate.FinishReason, candidate.Content.Parts),
	}, nil
}

// --- wire shapes -----------------------------------------------------------

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Tools             []geminiTool    `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

func encodeGeminiContents(messages []Message) []geminiContent {
	out := make([]geminiContent, 0, len(messages))
	for _, message := range messages {
		role := "user"
		if message.Role == RoleAssistant {
			role = "model"
		}
		parts := make([]geminiPart, 0, len(message.Content))
		for _, block := range message.Content {
			switch block.Type {
			case BlockText:
				parts = append(parts, geminiPart{Text: block.Text})
			case BlockToolUse:
				parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{
					Name: block.ToolName,
					Args: defaultJSONObject(block.Input),
				}})
			case BlockToolResult:
				parts = append(parts, geminiPart{FunctionResponse: &geminiFunctionResponse{
					Name:     block.ToolUseID,
					Response: encodeGeminiToolResult(block.Output, block.IsError),
				}})
			}
		}
		out = append(out, geminiContent{Role: role, Parts: parts})
	}
	return out
}

func encodeGeminiTools(tools []ToolSpec) []geminiTool {
	if len(tools) == 0 {
		return nil
	}
	declarations := make([]geminiFunctionDecl, 0, len(tools))
	for _, tool := range tools {
		declarations = append(declarations, geminiFunctionDecl{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  defaultJSONObject(tool.InputSchema),
		})
	}
	return []geminiTool{{FunctionDeclarations: declarations}}
}

func encodeGeminiToolResult(output string, isError bool) json.RawMessage {
	payload := map[string]any{"output": output}
	if isError {
		payload["error"] = true
	}
	encoded, _ := json.Marshal(payload)
	return encoded
}

func decodeGeminiParts(parts []geminiPart) []ContentBlock {
	out := make([]ContentBlock, 0, len(parts))
	for _, part := range parts {
		if part.Text != "" {
			out = append(out, TextBlock(part.Text))
			continue
		}
		if part.FunctionCall != nil {
			out = append(out, ToolUseBlock(part.FunctionCall.Name, part.FunctionCall.Name, defaultJSONObject(part.FunctionCall.Args)))
		}
	}
	return out
}

func mapGeminiStop(reason string, parts []geminiPart) string {
	for _, part := range parts {
		if part.FunctionCall != nil {
			return StopReasonToolUse
		}
	}
	switch reason {
	case "STOP":
		return StopReasonEndTurn
	case "MAX_TOKENS":
		return StopReasonMaxTokens
	default:
		if reason == "" {
			return StopReasonEndTurn
		}
		return reason
	}
}
