package runtime

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/agentsdk"
	"github.com/invariantcontinuum/agentctl/internal/logging"
	"github.com/invariantcontinuum/agentctl/internal/mcp"
)

// Runtime hosts a single agent instance behind the /health, /status,
// /tasks, /tasks/{id} HTTP contract. It composes an agentsdk.ModelClient
// (provider-aware), a tool Registry built from MCP servers and skills, an
// in-memory task Store, and a single worker goroutine that drives the
// agent loop for each submitted task.
type Runtime struct {
	config       agent.Config
	store        *Store
	model        agentsdk.ModelClient
	systemPrompt string
	mcpClient    agentsdk.MCPClient
	tools        []agentsdk.Tool
	logger       *logging.Logger
	address      string

	server *http.Server

	startedAt time.Time
}

// Options configure construction. Sensible zero-value defaults apply to
// every field; the call site can override individual pieces for tests.
type Options struct {
	Address         string
	Logger          *logging.Logger
	Model           agentsdk.ModelClient
	MCPClient       agentsdk.MCPClient
	Tools           []agentsdk.Tool
	SkillReader     func(path string) (string, error)
	Capacity        int
	RetainTerminals int
}

// New builds a Runtime. The model client falls back to provider-driven
// auto-detection when Options.Model is nil. MCP discovery happens in
// Start so a slow server doesn't block construction.
func New(config agent.Config, options Options) *Runtime {
	logger := options.Logger
	if logger == nil {
		logger = logging.New(os.Stderr, logging.LevelInfo)
	}
	address := options.Address
	if address == "" {
		address = httpEndpointAddress(config)
	}
	model := options.Model
	if model == nil {
		model = clientForConfig(config)
	}
	mcpClient := options.MCPClient
	if mcpClient == nil {
		mcpClient = mcp.NewClient(10 * time.Second)
	}
	skillReader := options.SkillReader
	if skillReader == nil {
		skillReader = defaultSkillReader
	}

	return &Runtime{
		config:       config,
		store:        NewStore(options.Capacity, options.RetainTerminals),
		model:        model,
		systemPrompt: assembleSystemPrompt(config, skillReader, logger),
		mcpClient:    mcpClient,
		tools:        append([]agentsdk.Tool{}, options.Tools...),
		logger:       logger,
		address:      address,
		startedAt:    time.Now().UTC(),
	}
}

// Address returns the bound HTTP listen address.
func (r *Runtime) Address() string { return r.address }

// Start launches the worker and the HTTP server. It blocks until ctx is
// cancelled or the server returns an error other than ErrServerClosed.
func (r *Runtime) Start(ctx context.Context) error {
	if r.address == "" {
		return errors.New("runtime address is empty")
	}

	r.discoverMCPTools(ctx)

	go r.runWorker(ctx)

	r.server = &http.Server{
		Addr:              r.address,
		Handler:           r.handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	r.logger.Info("agentd listening",
		"addr", r.address,
		"agent", r.config.Name,
		"provider", r.model.Provider(),
		"tools", strconv.Itoa(len(r.tools)),
	)

	listenError := make(chan error, 1)
	go func() {
		err := r.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenError <- err
			return
		}
		listenError <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = r.server.Shutdown(shutdownCtx)
		r.store.Close()
		<-listenError
		return nil
	case err := <-listenError:
		r.store.Close()
		return err
	}
}

func (r *Runtime) discoverMCPTools(ctx context.Context) {
	if len(r.config.MCPServers) == 0 || r.mcpClient == nil {
		return
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	discovered, err := agentsdk.DiscoverMCPTools(discoveryCtx, r.mcpClient, r.config.MCPServers, func(server agent.MCPServer, err error) {
		r.logger.Warn("mcp discovery failed", "server", server.Name, "error", err.Error())
	})
	if err != nil {
		r.logger.Error("mcp discovery", "error", err.Error())
		return
	}
	r.tools = append(r.tools, discovered...)
}

func (r *Runtime) runWorker(ctx context.Context) {
	for {
		task, ok := r.store.Next()
		if !ok {
			return
		}
		select {
		case <-ctx.Done():
			r.logTaskStoreError("mark error (ctx cancelled)", task.ID, r.store.MarkError(task.ID, ctx.Err()))
			return
		default:
		}
		r.logTaskStoreError("mark running", task.ID, r.store.MarkRunning(task.ID))
		r.logger.Info("task running", "id", task.ID, "provider", r.model.Provider())

		runner := r.buildAgent(task.System)
		session := agentsdk.NewMemorySession(task.ID)
		result, err := runner.Run(ctx, session, task.Prompt)
		if err != nil {
			r.logTaskStoreError("mark error", task.ID, r.store.MarkError(task.ID, err))
			r.logger.Error("task failed", "id", task.ID, "error", err.Error())
			continue
		}
		r.logTaskStoreError("mark done", task.ID, r.store.MarkDone(task.ID, result.Final))
		r.logger.Info("task done",
			"id", task.ID,
			"steps", strconv.Itoa(result.Steps),
			"len", strconv.Itoa(len(result.Final)),
		)
	}
}

// logTaskStoreError logs but does not propagate task-store transition
// errors (e.g. "task not found") because the worker has nowhere meaningful
// to surface them — the caller is the queue. Visibility through the agent
// log is enough.
func (r *Runtime) logTaskStoreError(operation string, taskID string, err error) {
	if err == nil {
		return
	}
	r.logger.Warn("task store transition failed",
		"op", operation,
		"id", taskID,
		"error", err.Error(),
	)
}

func (r *Runtime) buildAgent(taskSystem string) *agentsdk.Agent {
	system := r.systemPrompt
	if taskSystem != "" {
		if system == "" {
			system = taskSystem
		} else {
			system = taskSystem + "\n\n" + system
		}
	}
	a := agentsdk.NewAgent(r.config.Name, r.model)
	a.ModelName = r.config.Model.Name
	a.System = system
	if r.config.Loop.MaxSteps > 0 {
		a.MaxSteps = r.config.Loop.MaxSteps
	}
	a.Tools.Register(r.tools...)
	return a
}

// clientForConfig picks an agentsdk.ModelClient based on agent.Config and
// os.Environ. The credential is read from config.Model.APIKeyEnv first,
// then falls back to the conventional default env name. Providers without
// a usable key fall back to the deterministic Echo client so smoke runs
// still cycle the loop.
func clientForConfig(config agent.Config) agentsdk.ModelClient {
	provider := strings.ToLower(strings.TrimSpace(config.Model.Provider))
	endpoint := config.Model.BaseURL
	model := config.Model.Name
	apiKey := lookupAPIKey(config)

	switch provider {
	case "openai", "vllm", "llamacpp":
		return agentsdk.NewOpenAIClient(provider, endpoint, apiKey, model, nil)
	case "anthropic":
		if apiKey == "" {
			return agentsdk.NewEchoClient("anthropic")
		}
		return agentsdk.NewAnthropicClient(endpoint, apiKey, model, nil)
	case "gemini":
		if apiKey == "" {
			return agentsdk.NewEchoClient("gemini")
		}
		return agentsdk.NewGeminiClient(endpoint, apiKey, model, nil)
	default:
		return agentsdk.NewEchoClient(provider)
	}
}

func lookupAPIKey(config agent.Config) string {
	if envName := strings.TrimSpace(config.Model.APIKeyEnv); envName != "" {
		if value := os.Getenv(envName); value != "" {
			return value
		}
	}
	switch strings.ToLower(config.Model.Provider) {
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "anthropic":
		return os.Getenv("ANTHROPIC_API_KEY")
	case "gemini":
		return os.Getenv("GEMINI_API_KEY")
	}
	return ""
}

func httpEndpointAddress(config agent.Config) string {
	for _, endpoint := range config.Endpoints {
		if strings.EqualFold(endpoint.Name, "http") {
			return agent.EndpointHostPort(endpoint)
		}
	}
	return "127.0.0.1:8088"
}

// assembleSystemPrompt concatenates the contents of every SKILL file
// declared on the agent. A missing file is logged and skipped — agentctl
// prefers a partial system prompt over a startup failure.
func assembleSystemPrompt(config agent.Config, read func(path string) (string, error), logger *logging.Logger) string {
	parts := make([]string, 0, len(config.Skills)+1)
	if config.Type != "" {
		parts = append(parts, "You are an agent of role: "+config.Type+".")
	}
	for _, skill := range config.Skills {
		if !skill.Enabled {
			continue
		}
		source := strings.TrimSpace(skill.Path)
		if source == "" {
			source = strings.TrimSpace(skill.Content)
		}
		if source == "" {
			continue
		}
		if skill.Type == "builtin" || strings.HasPrefix(source, "builtin://") {
			parts = append(parts, "Skill: "+source)
			continue
		}
		if skill.Content != "" && skill.Path == "" {
			parts = append(parts, skill.Content)
			continue
		}
		body, err := read(source)
		if err != nil {
			logger.Warn("skill read failed", "source", source, "error", err.Error())
			continue
		}
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n")
}

func defaultSkillReader(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
