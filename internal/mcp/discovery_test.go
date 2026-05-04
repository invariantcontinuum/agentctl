package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type stubHTTPClient struct {
	respond func(*http.Request) (*http.Response, error)
}

func (s stubHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return s.respond(request)
}

func newJSONResponse(status int, payload string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(payload)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestListToolsDecodesAndSorts(t *testing.T) {
	stub := stubHTTPClient{
		respond: func(*http.Request) (*http.Response, error) {
			body := `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"search"},{"name":"code"}]}}`
			return newJSONResponse(200, body), nil
		},
	}
	client := NewClientWithHTTPClient(stub)
	tools, err := client.ListTools(context.Background(), "http://localhost:9001/mcp")
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if len(tools) != 2 || tools[0].Name != "code" || tools[1].Name != "search" {
		t.Fatalf("tools = %+v, want [code search]", tools)
	}
}

func TestListToolsReturnsRPCError(t *testing.T) {
	stub := stubHTTPClient{
		respond: func(*http.Request) (*http.Response, error) {
			return newJSONResponse(200, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`), nil
		},
	}
	client := NewClientWithHTTPClient(stub)
	if _, err := client.ListTools(context.Background(), "http://localhost:9001/mcp"); err == nil || !strings.Contains(err.Error(), "method not found") {
		t.Fatalf("err = %v, want method not found", err)
	}
}

func TestCallEncodesArguments(t *testing.T) {
	var captured map[string]any
	stub := stubHTTPClient{
		respond: func(request *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
			var decoded struct {
				Method string         `json:"method"`
				Params map[string]any `json:"params"`
			}
			if err := json.Unmarshal(body, &decoded); err != nil {
				return nil, err
			}
			captured = decoded.Params
			return newJSONResponse(200, `{"jsonrpc":"2.0","id":1,"result":{"isError":false,"content":[{"type":"text","text":"ok"}]}}`), nil
		},
	}
	client := NewClientWithHTTPClient(stub)
	result, err := client.Call(context.Background(), "http://localhost:9001/mcp", "search", map[string]any{"q": "agents"})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	if captured["name"] != "search" {
		t.Fatalf("captured name = %v, want search", captured["name"])
	}
	if result.IsError {
		t.Fatalf("result.IsError = true, want false")
	}
}

func TestCallRequiresName(t *testing.T) {
	client := NewClientWithHTTPClient(stubHTTPClient{respond: func(*http.Request) (*http.Response, error) { return nil, errors.New("must not be called") }})
	if _, err := client.Call(context.Background(), "http://x", "", nil); err == nil {
		t.Fatal("Call returned nil error for empty tool name")
	}
}

func TestCallRejectsHTTPError(t *testing.T) {
	stub := stubHTTPClient{
		respond: func(*http.Request) (*http.Response, error) {
			return newJSONResponse(500, "boom"), nil
		},
	}
	client := NewClientWithHTTPClient(stub)
	if _, err := client.Call(context.Background(), "http://localhost:9001/mcp", "search", nil); err == nil || !strings.Contains(err.Error(), "mcp http 500") {
		t.Fatalf("err = %v, want mcp http 500", err)
	}
}
