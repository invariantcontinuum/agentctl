package agentsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripper struct {
	requests  []*http.Request
	bodies    [][]byte
	response  *http.Response
	respondFn func(*http.Request) *http.Response
	err       error
}

func (r *roundTripper) Do(request *http.Request) (*http.Response, error) {
	r.requests = append(r.requests, request)
	body := []byte{}
	if request.Body != nil {
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		body = raw
		request.Body.Close()
	}
	r.bodies = append(r.bodies, body)
	if r.err != nil {
		return nil, r.err
	}
	if r.respondFn != nil {
		return r.respondFn(request), nil
	}
	return r.response, nil
}

func makeResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     http.Header{},
	}
}

func TestEchoClientReturnsLatestUserText(t *testing.T) {
	client := NewEchoClient("local")
	response, err := client.Generate(context.Background(), GenerateRequest{
		Messages: []Message{
			UserMessage("first"),
			AssistantMessage(TextBlock("answer")),
			UserMessage("second"),
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if response.Content[0].Text != "[echo] second" {
		t.Fatalf("text = %q", response.Content[0].Text)
	}
	if response.StopReason != StopReasonEndTurn {
		t.Fatalf("stop = %q", response.StopReason)
	}
}

func TestAnthropicClientSendsExpectedShape(t *testing.T) {
	rt := &roundTripper{response: makeResponse(200, `{"content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn"}`)}
	client := NewAnthropicClient("https://api.example.com", "sk", "claude-x", rt)
	response, err := client.Generate(context.Background(), GenerateRequest{
		System:   "be brief",
		Messages: []Message{UserMessage("hello")},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if response.Content[0].Text != "hi" {
		t.Fatalf("text = %q", response.Content[0].Text)
	}
	if rt.requests[0].Header.Get("x-api-key") != "sk" {
		t.Fatalf("missing x-api-key")
	}
	if !strings.Contains(string(rt.bodies[0]), `"system":"be brief"`) {
		t.Fatalf("system not included in body: %s", rt.bodies[0])
	}
}

func TestAnthropicClientDecodesToolUse(t *testing.T) {
	rt := &roundTripper{response: makeResponse(200,
		`{"content":[{"type":"tool_use","id":"tu_1","name":"calc","input":{"x":1}}],"stop_reason":"tool_use"}`)}
	client := NewAnthropicClient("https://api.example.com", "sk", "claude-x", rt)
	response, err := client.Generate(context.Background(), GenerateRequest{Messages: []Message{UserMessage("go")}})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if response.StopReason != StopReasonToolUse {
		t.Fatalf("stop = %q", response.StopReason)
	}
	if response.Content[0].ToolName != "calc" || response.Content[0].ToolUseID != "tu_1" {
		t.Fatalf("tool_use not decoded: %+v", response.Content[0])
	}
}

func TestOpenAIClientSerializesToolCallsAndDecodes(t *testing.T) {
	rt := &roundTripper{response: makeResponse(200,
		`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"calc","arguments":"{\"x\":1}"}}]},"finish_reason":"tool_calls"}]}`)}
	client := NewOpenAIClient("openai", "https://api.example.com/v1", "secret", "gpt-x", rt)

	tools := []ToolSpec{{Name: "calc", Description: "math", InputSchema: json.RawMessage(`{"type":"object"}`)}}
	response, err := client.Generate(context.Background(), GenerateRequest{
		Messages: []Message{UserMessage("compute")},
		Tools:    tools,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if rt.requests[0].Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("missing bearer")
	}
	if !strings.Contains(string(rt.bodies[0]), `"tools":[`) {
		t.Fatalf("tools not in body: %s", rt.bodies[0])
	}
	if response.StopReason != StopReasonToolUse {
		t.Fatalf("stop = %q", response.StopReason)
	}
	if response.Content[0].ToolName != "calc" {
		t.Fatalf("tool_use decode failed: %+v", response.Content)
	}
}

func TestGeminiClientSendsFunctionDeclarations(t *testing.T) {
	rt := &roundTripper{response: makeResponse(200,
		`{"candidates":[{"content":{"parts":[{"text":"hi"}]},"finishReason":"STOP"}]}`)}
	client := NewGeminiClient("https://gen.example.com", "key", "gemini-x", rt)
	tools := []ToolSpec{{Name: "calc", Description: "m", InputSchema: json.RawMessage(`{"type":"object"}`)}}
	response, err := client.Generate(context.Background(), GenerateRequest{
		System:   "system",
		Messages: []Message{UserMessage("hi")},
		Tools:    tools,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if response.Content[0].Text != "hi" {
		t.Fatalf("text = %q", response.Content[0].Text)
	}
	body := string(rt.bodies[0])
	if !strings.Contains(body, `"functionDeclarations"`) {
		t.Fatalf("functionDeclarations missing: %s", body)
	}
	if !strings.Contains(body, `"systemInstruction"`) {
		t.Fatalf("systemInstruction missing: %s", body)
	}
}

func TestOpenAIClientErrorsOnHTTPError(t *testing.T) {
	rt := &roundTripper{response: makeResponse(401, `{"error":"unauthorized"}`)}
	client := NewOpenAIClient("openai", "https://api.example.com/v1", "k", "gpt-x", rt)
	if _, err := client.Generate(context.Background(), GenerateRequest{Messages: []Message{UserMessage("x")}}); err == nil {
		t.Fatalf("expected error on 401")
	}
}
