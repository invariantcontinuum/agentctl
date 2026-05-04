package agentsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// Tool is the Open/Closed extension point for adding new callable behaviour
// to an Agent. Implementations live in this package (FunctionTool, MCPTool)
// and in user code.
type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// ToolSpec is the wire-neutral declaration sent to the model so it knows
// what it may call. The Agent assembles one Spec per registered Tool.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// SpecOf returns a ToolSpec describing tool. Centralising this conversion
// keeps the providers agnostic to whether a Tool came from MCP, a Go func,
// or somewhere else.
func SpecOf(tool Tool) ToolSpec {
	return ToolSpec{
		Name:        tool.Name(),
		Description: tool.Description(),
		InputSchema: tool.InputSchema(),
	}
}

// Registry is a name -> Tool lookup the Agent uses to dispatch tool_use
// blocks. It is safe for concurrent use.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// Register stores tool. Registering a duplicate name overrides the previous
// entry so callers can replace defaults at construction time.
func (r *Registry) Register(tools ...Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, tool := range tools {
		if tool == nil || tool.Name() == "" {
			continue
		}
		r.tools[tool.Name()] = tool
	}
}

// Get returns the named Tool or false.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// Specs returns ToolSpecs in deterministic alphabetical order so a model
// receives the same tool list across runs.
func (r *Registry) Specs() []ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	specs := make([]ToolSpec, 0, len(names))
	for _, name := range names {
		specs = append(specs, SpecOf(r.tools[name]))
	}
	return specs
}

// Tools returns every tool in the registry, alphabetically.
func (r *Registry) Tools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Tool, 0, len(names))
	for _, name := range names {
		out = append(out, r.tools[name])
	}
	return out
}

// FunctionTool wraps a Go function so it can be called as a Tool.
type FunctionTool struct {
	name        string
	description string
	schema      json.RawMessage
	fn          func(ctx context.Context, input json.RawMessage) (string, error)
}

// NewFunctionTool constructs a FunctionTool. schema must be a JSON Schema
// object describing the input; passing nil substitutes an "any object"
// placeholder so a model can still call the tool.
func NewFunctionTool(name string, description string, schema json.RawMessage, fn func(ctx context.Context, input json.RawMessage) (string, error)) *FunctionTool {
	if schema == nil {
		schema = json.RawMessage(`{"type":"object"}`)
	}
	return &FunctionTool{name: name, description: description, schema: schema, fn: fn}
}

// Name implements Tool.
func (t *FunctionTool) Name() string { return t.name }

// Description implements Tool.
func (t *FunctionTool) Description() string { return t.description }

// InputSchema implements Tool.
func (t *FunctionTool) InputSchema() json.RawMessage { return t.schema }

// Execute implements Tool. A nil function returns an error so a misconfigured
// tool fails loud rather than silently emitting empty output.
func (t *FunctionTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	if t.fn == nil {
		return "", fmt.Errorf("tool %q has no executor", t.name)
	}
	return t.fn(ctx, input)
}
