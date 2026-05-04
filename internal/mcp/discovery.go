// Package mcp provides a minimal client for Model Context Protocol servers.
//
// Only the operations agentctl uses today are implemented: tools/list to
// discover the tool catalog and tools/call to invoke one. The transport is
// HTTP+JSON-RPC 2.0 because every MCP server agentctl is expected to talk to
// already exposes that surface, and using net/http keeps the dependency
// footprint to the standard library.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

// Tool describes one entry returned from a tools/list call.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// CallResult is the structured response of tools/call.
type CallResult struct {
	IsError bool            `json:"isError,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
}

// HTTPClient is the minimal interface the Client depends on so tests can
// inject a fake transport.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Client speaks JSON-RPC 2.0 against an MCP server URL.
type Client struct {
	client HTTPClient
}

// NewClient returns a Client backed by an http.Client with a sensible timeout.
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{client: &http.Client{Timeout: timeout}}
}

// NewClientWithHTTPClient is the dependency-injected variant for tests.
func NewClientWithHTTPClient(client HTTPClient) *Client {
	return &Client{client: client}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e rpcError) Error() string { return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message) }

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// ListTools calls tools/list against url and returns the discovered Tools
// sorted by name for deterministic display.
func (c *Client) ListTools(ctx context.Context, url string) ([]Tool, error) {
	var raw struct {
		Tools []Tool `json:"tools"`
	}
	if err := c.call(ctx, url, "tools/list", nil, &raw); err != nil {
		return nil, err
	}
	sort.Slice(raw.Tools, func(left, right int) bool {
		return raw.Tools[left].Name < raw.Tools[right].Name
	})
	return raw.Tools, nil
}

// Call invokes tools/call with the supplied arguments and returns the parsed
// CallResult.
func (c *Client) Call(ctx context.Context, url string, name string, arguments map[string]any) (CallResult, error) {
	if name == "" {
		return CallResult{}, errors.New("tool name is required")
	}
	params := map[string]any{"name": name}
	if arguments != nil {
		params["arguments"] = arguments
	}
	var result CallResult
	if err := c.call(ctx, url, "tools/call", params, &result); err != nil {
		return CallResult{}, err
	}
	return result, nil
}

func (c *Client) call(ctx context.Context, url string, method string, params any, target any) error {
	if url == "" {
		return errors.New("MCP url is required")
	}
	payload, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("mcp http %d: %s", response.StatusCode, bytes.TrimSpace(body))
	}

	var envelope rpcResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode mcp response: %w", err)
	}
	if envelope.Error != nil {
		return *envelope.Error
	}
	if target == nil {
		return nil
	}
	if len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, target)
}
