// Package mcp provides a minimal client for Model Context Protocol servers.
//
// Two transports are implemented, matching the Anthropic/OpenAI agent SDK
// MCP shapes:
//
//   - http: POST JSON-RPC 2.0 to a URL.
//   - stdio: spawn a child process and exchange newline-delimited JSON-RPC
//     messages over its stdin/stdout.
//
// Only operations agentctl needs today are implemented: tools/list to
// discover the tool catalog and tools/call to invoke one. Both transports
// are stdlib-only.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strings"
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

// Transports recognized by the Client.
const (
	TransportHTTP  = "http"
	TransportStdio = "stdio"
)

// ServerSpec describes one MCP target. It mirrors the Agentfile MCP directive
// without coupling the package to internal/agent.
type ServerSpec struct {
	Name      string
	Transport string
	URL       string
	Command   string
	Args      []string
	Env       map[string]string
}

// CommandRunner is the minimal surface needed to spawn a stdio MCP child.
// Tests inject a fake to avoid actually starting a process.
type CommandRunner interface {
	Run(ctx context.Context, command string, args []string, env map[string]string, request []byte) ([]byte, error)
}

// Client speaks JSON-RPC 2.0 against an MCP server (http or stdio).
type Client struct {
	httpClient HTTPClient
	stdio      CommandRunner
}

// NewClient returns a Client backed by an http.Client with a sensible timeout
// and a real subprocess CommandRunner for stdio transports.
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		stdio:      &execCommandRunner{timeout: timeout},
	}
}

// NewClientWithHTTPClient is the dependency-injected HTTP variant for tests.
func NewClientWithHTTPClient(client HTTPClient) *Client {
	return &Client{httpClient: client, stdio: &execCommandRunner{}}
}

// NewClientWithRunners injects both transports — used by tests covering stdio.
func NewClientWithRunners(httpClient HTTPClient, stdio CommandRunner) *Client {
	return &Client{httpClient: httpClient, stdio: stdio}
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

// ListTools calls tools/list against the chosen transport and returns the
// discovered Tools sorted by name for deterministic display.
func (c *Client) ListTools(ctx context.Context, server ServerSpec) ([]Tool, error) {
	var raw struct {
		Tools []Tool `json:"tools"`
	}
	if err := c.call(ctx, server, "tools/list", nil, &raw); err != nil {
		return nil, err
	}
	sort.Slice(raw.Tools, func(left, right int) bool {
		return raw.Tools[left].Name < raw.Tools[right].Name
	})
	return raw.Tools, nil
}

// Call invokes tools/call with the supplied arguments and returns the parsed
// CallResult.
func (c *Client) Call(ctx context.Context, server ServerSpec, name string, arguments map[string]any) (CallResult, error) {
	if name == "" {
		return CallResult{}, errors.New("tool name is required")
	}
	params := map[string]any{"name": name}
	if arguments != nil {
		params["arguments"] = arguments
	}
	var result CallResult
	if err := c.call(ctx, server, "tools/call", params, &result); err != nil {
		return CallResult{}, err
	}
	return result, nil
}

func (c *Client) call(ctx context.Context, server ServerSpec, method string, params any, target any) error {
	payload, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}

	body, err := c.exchange(ctx, server, payload)
	if err != nil {
		return err
	}

	var envelope rpcResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode mcp response: %w", err)
	}
	if envelope.Error != nil {
		return *envelope.Error
	}
	if target == nil || len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, target)
}

func (c *Client) exchange(ctx context.Context, server ServerSpec, payload []byte) ([]byte, error) {
	switch server.Transport {
	case TransportHTTP, "":
		if server.URL == "" {
			return nil, errors.New("MCP http transport requires a URL")
		}
		return c.exchangeHTTP(ctx, server.URL, payload)
	case TransportStdio:
		if server.Command == "" {
			return nil, errors.New("MCP stdio transport requires a Command")
		}
		return c.stdio.Run(ctx, server.Command, server.Args, server.Env, payload)
	}
	return nil, fmt.Errorf("MCP unknown transport %q", server.Transport)
}

func (c *Client) exchangeHTTP(ctx context.Context, url string, payload []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("mcp http %d: %s", response.StatusCode, bytes.TrimSpace(body))
	}
	return body, nil
}

// execCommandRunner spawns the MCP server, writes one newline-delimited
// JSON-RPC request to stdin, reads the first non-empty response line from
// stdout, then closes stdin so the child exits.
type execCommandRunner struct {
	timeout time.Duration
}

func (r *execCommandRunner) Run(ctx context.Context, command string, args []string, env map[string]string, request []byte) ([]byte, error) {
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = mergeEnv(env)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	requestLine := append(append([]byte{}, request...), '\n')
	writeErr := writeAndCloseStdin(stdin, requestLine)

	response, readErr := readFirstJSONLine(stdout)

	waitErr := cmd.Wait()

	if writeErr != nil {
		return nil, writeErr
	}
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil && len(response) == 0 {
		return nil, fmt.Errorf("mcp stdio %s: %w", command, waitErr)
	}
	return response, nil
}

func writeAndCloseStdin(stdin io.WriteCloser, payload []byte) error {
	defer stdin.Close()
	_, err := stdin.Write(payload)
	return err
}

func readFirstJSONLine(stdout io.Reader) ([]byte, error) {
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadBytes('\n')
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			return trimmed, nil
		}
		if err != nil {
			if err == io.EOF {
				return nil, errors.New("mcp stdio produced no response")
			}
			return nil, err
		}
	}
}

func mergeEnv(extra map[string]string) []string {
	if len(extra) == 0 {
		return nil
	}
	merged := make([]string, 0, len(extra))
	for key, value := range extra {
		merged = append(merged, key+"="+value)
	}
	sort.Strings(merged)
	return merged
}

// allow strings import to remain referenced if unused symbols change.
var _ = strings.TrimSpace
