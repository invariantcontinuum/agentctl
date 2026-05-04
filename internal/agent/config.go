package agent

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Config struct {
	Image        string            `json:"image,omitempty"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Model        Model             `json:"model,omitempty"`
	Skills       []Skill           `json:"skills,omitempty"`
	MCPServers   []MCPServer       `json:"mcp_servers,omitempty"`
	VectorStores []RAGSource       `json:"vector_stores,omitempty"`
	GraphStores  []RAGSource       `json:"graph_stores,omitempty"`
	Memories     []Memory          `json:"memories,omitempty"`
	Loop         Loop              `json:"loop"`
	Endpoints    []Endpoint        `json:"endpoints,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Exec         []string          `json:"exec"`
}

type Skill struct {
	ID           string         `json:"id,omitempty"`
	Name         string         `json:"name"`
	Type         string         `json:"type,omitempty"`
	Path         string         `json:"path,omitempty"`
	Content      string         `json:"content,omitempty"`
	Dependencies []string       `json:"dependencies,omitempty"`
	Enabled      bool           `json:"enabled,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type Model struct {
	Provider   string         `json:"provider,omitempty"`
	Name       string         `json:"name"`
	BaseURL    string         `json:"base_url,omitempty"`
	APIKeyEnv  string         `json:"api_key_env,omitempty"`
	Auth       string         `json:"auth,omitempty"`
	TimeoutSec int            `json:"timeout_sec,omitempty"`
	Options    map[string]any `json:"options,omitempty"`
}

// MCPServer is the canonical descriptor for one MCP-compatible tool server.
// A server can be reached over HTTP through URL/BasePath, launched over stdio
// through Command/Args, or carry both pieces of metadata for CLI-managed
// subprocesses that later expose an HTTP endpoint.
type MCPServer struct {
	Name       string            `json:"name"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	URL        string            `json:"url,omitempty"`
	BasePath   string            `json:"base_path,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Tools      []MCPTool         `json:"tools,omitempty"`
	TimeoutSec int               `json:"timeout_sec,omitempty"`
	Enabled    bool              `json:"enabled,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

type MCPTool struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
	Category     string         `json:"category,omitempty"`
	Enabled      bool           `json:"enabled,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type RAGSource struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Provider       string            `json:"provider"`
	URL            string            `json:"url,omitempty"`
	Index          string            `json:"index,omitempty"`
	EmbeddingModel string            `json:"embedding_model,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Weight         float64           `json:"weight,omitempty"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
}

type Memory struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Provider string            `json:"provider"`
	URL      string            `json:"url,omitempty"`
	Bucket   string            `json:"bucket,omitempty"`
	Limit    int               `json:"limit,omitempty"`
	TTLSec   int               `json:"ttl_sec,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

type Loop struct {
	Name          string           `json:"name"`
	MaxSteps      int              `json:"max_steps,omitempty"`
	MaxTokens     int              `json:"max_tokens,omitempty"`
	ToolSelection string           `json:"tool_selection,omitempty"`
	PreHooks      []Hook           `json:"pre_hooks,omitempty"`
	PostHooks     []Hook           `json:"post_hooks,omitempty"`
	Evaluation    Evaluation       `json:"evaluation,omitempty"`
	MultiAgent    MultiAgentConfig `json:"multi_agent,omitempty"`
}

type Hook struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	URL        string            `json:"url,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	TimeoutSec int               `json:"timeout_sec,omitempty"`
	OnError    string            `json:"on_error,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type Evaluation struct {
	MaxErrors          int       `json:"max_errors,omitempty"`
	ToolAllowList      []string  `json:"tool_allow_list,omitempty"`
	ToolDenyList       []string  `json:"tool_deny_list,omitempty"`
	ValidatorTools     []MCPTool `json:"validator_tools,omitempty"`
	LogFilter          []string  `json:"log_filter,omitempty"`
	CompletionCriteria []string  `json:"completion_criteria,omitempty"`
}

type MultiAgentConfig struct {
	Enabled      bool           `json:"enabled"`
	Coordinator  string         `json:"coordinator,omitempty"`
	AllowedRoles []string       `json:"allowed_roles,omitempty"`
	Delegation   string         `json:"delegation,omitempty"`
	Policy       map[string]any `json:"policy,omitempty"`
}

type Endpoint struct {
	Name   string            `json:"name"`
	Scheme string            `json:"scheme"`
	Host   string            `json:"host,omitempty"`
	Port   int               `json:"port,omitempty"`
	Path   string            `json:"path,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

func (c Config) Command() (string, []string) {
	if len(c.Exec) == 0 {
		return "", nil
	}
	return c.Exec[0], c.Exec[1:]
}

func (c Config) EnvList(base []string) []string {
	if len(base) == 0 {
		base = os.Environ()
	}
	if len(c.Env) == 0 {
		return base
	}

	keys := make([]string, 0, len(c.Env))
	for key := range c.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := append([]string{}, base...)
	for _, key := range keys {
		env = append(env, key+"="+c.Env[key])
	}
	return env
}

func EndpointFromURL(name string, raw string) (Endpoint, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return Endpoint{}, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return Endpoint{}, fmt.Errorf("endpoint URL must include scheme and host")
	}
	host := parsed.Hostname()
	port := 0
	if parsed.Port() != "" {
		parsedPort, err := strconv.Atoi(parsed.Port())
		if err != nil {
			return Endpoint{}, err
		}
		port = parsedPort
	}
	return Endpoint{
		Name:   name,
		Scheme: parsed.Scheme,
		Host:   host,
		Port:   port,
		Path:   parsed.Path,
	}, nil
}

func EndpointURL(endpoint Endpoint) string {
	if endpoint.Scheme == "" || endpoint.Host == "" {
		return ""
	}
	host := endpoint.Host
	if endpoint.Port > 0 {
		host = net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.Port))
	}
	path := endpoint.Path
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return endpoint.Scheme + "://" + host + path
}

func EndpointHostPort(endpoint Endpoint) string {
	if endpoint.Host == "" {
		return ""
	}
	if endpoint.Port > 0 {
		return net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.Port))
	}
	return endpoint.Host
}
