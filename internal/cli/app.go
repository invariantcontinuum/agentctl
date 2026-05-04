package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/agentfile"
	"github.com/invariantcontinuum/agentctl/internal/catalog"
	"github.com/invariantcontinuum/agentctl/internal/compose"
	"github.com/invariantcontinuum/agentctl/internal/credentials"
	"github.com/invariantcontinuum/agentctl/internal/driver"
	"github.com/invariantcontinuum/agentctl/internal/health"
	"github.com/invariantcontinuum/agentctl/internal/logging"
	"github.com/invariantcontinuum/agentctl/internal/mcp"
	"github.com/invariantcontinuum/agentctl/internal/model"
	"github.com/invariantcontinuum/agentctl/internal/store"
	"github.com/invariantcontinuum/agentctl/internal/trace"
)

type App struct {
	out            io.Writer
	errOut         io.Writer
	stdin          io.Reader
	parser         agentfile.Parser
	composeParser  compose.Parser
	validator      agent.Validator
	repo           store.Repository
	driver         driver.Driver
	images         catalog.Catalog
	models         model.Catalog
	credentials    credentials.Store
	now            func() time.Time
	paths          func(string) (string, string, string, error)
	healthProbeFor func(string) *health.Probe
	mcpClientFor   func(string) *mcp.Client
}

func New(out io.Writer, errOut io.Writer, repo store.Repository, runtimeDriver driver.Driver) *App {
	return &App{
		out:            out,
		errOut:         errOut,
		parser:         agentfile.NewParser(),
		composeParser:  compose.NewParser(),
		validator:      agent.ConfigValidator{},
		repo:           repo,
		driver:         runtimeDriver,
		images:         catalog.DefaultCatalog(),
		models:         model.DefaultCatalog(),
		credentials:    defaultCredentials(),
		now:            time.Now,
		paths:          runtimePaths,
		healthProbeFor: func(string) *health.Probe { return health.NewProbe(5*time.Second, 0) },
		mcpClientFor:   func(string) *mcp.Client { return mcp.NewClient(10 * time.Second) },
	}
}

// defaultCredentials returns the file-backed Store. If the XDG path can't be
// resolved (very unusual: read-only HOME), we fall back to an in-memory map
// implemented as a no-op JSONStore at /dev/null so the CLI still loads.
func defaultCredentials() credentials.Store {
	path, err := credentials.DefaultPath()
	if err != nil {
		return credentials.NewJSONStore("")
	}
	return credentials.NewJSONStore(path)
}

func (a *App) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		a.printHelp()
		return 0
	}

	var err error
	switch args[0] {
	case "run":
		err = a.runAgent(ctx, args[1:])
	case "ps":
		err = a.listAgents(ctx, args[1:])
	case "agent", "agents":
		err = a.agents(ctx, args[1:])
	case "model", "models":
		err = a.modelsCommand(args[1:])
	case "logs":
		err = a.logs(args[1:])
	case "rm":
		err = a.removeAgents(ctx, args[1:])
	case "stop":
		err = a.stopAgent(ctx, args[1:])
	case "start":
		err = a.startAgent(ctx, args[1:])
	case "restart":
		err = a.restartAgent(ctx, args[1:])
	case "inspect":
		err = a.inspect(args[1:])
	case "describe":
		err = a.describe(ctx, args[1:])
	case "list-skills":
		err = a.listSkills(args[1:])
	case "list-tools":
		err = a.listTools(args[1:])
	case "skill", "skills":
		err = a.skills(args[1:])
	case "tool", "tools":
		err = a.tools(ctx, args[1:])
	case "trace":
		err = a.trace(args[1:])
	case "compose":
		err = a.compose(ctx, args[1:])
	case "exec":
		err = a.exec(ctx, args[1:])
	case "health":
		err = a.health(ctx, args[1:])
	case "rag":
		err = a.rag(args[1:])
	case "memory":
		err = a.memory(args[1:])
	case "loop":
		err = a.loop(ctx, args[1:])
	case "guard":
		err = a.guard(args[1:])
	case "help", "-h", "--help":
		a.printHelp()
		return 0
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}

	if err != nil {
		fmt.Fprintf(a.errOut, "agentctl: %v\n", err)
		return 1
	}
	return 0
}

func (a *App) printHelp() {
	fmt.Fprintln(a.out, "Usage: agentctl <command> [options]")
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Lifecycle:")
	fmt.Fprintln(a.out, "  run          Start an agent from an Agentfile or image")
	fmt.Fprintln(a.out, "  ps           List agents")
	fmt.Fprintln(a.out, "  logs         Print an agent log (--level debug|info|warn|error)")
	fmt.Fprintln(a.out, "  rm           Remove stopped agent state")
	fmt.Fprintln(a.out, "  stop         Stop an agent process")
	fmt.Fprintln(a.out, "  start        Start a stopped agent")
	fmt.Fprintln(a.out, "  restart      Restart an agent")
	fmt.Fprintln(a.out, "  inspect      Print agent configuration as JSON")
	fmt.Fprintln(a.out, "  describe     Print human-readable agent details")
	fmt.Fprintln(a.out, "  trace        Print structured lifecycle and reasoning events")
	fmt.Fprintln(a.out, "  exec         Invoke a tool against a running agent's MCP endpoint")
	fmt.Fprintln(a.out, "  health       Probe /health, /status, /tasks for an agent")
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Compose:")
	fmt.Fprintln(a.out, "  compose ls   List compose services")
	fmt.Fprintln(a.out, "  compose up   Start every agent declared in an AgentCompose file")
	fmt.Fprintln(a.out, "  compose down Stop and remove every agent in an AgentCompose file")
	fmt.Fprintln(a.out, "  compose ps   List running compose services")
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Management:")
	fmt.Fprintln(a.out, "  agent ls     Grouped form of ps")
	fmt.Fprintln(a.out, "  model ls     List model provider definitions")
	fmt.Fprintln(a.out, "  skill ls     List skills in one or more directories")
	fmt.Fprintln(a.out, "  tool ls      List configured MCP servers for an agent")
	fmt.Fprintln(a.out, "  tool mcp ls  Discover MCP tool schemas for an agent")
	fmt.Fprintln(a.out, "  tool exec    Run an MCP tool against an agent")
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Knowledge / Persistence / Control:")
	fmt.Fprintln(a.out, "  rag ls       List RAG sources for an agent (vector + graph)")
	fmt.Fprintln(a.out, "  memory ls    List memory bindings for an agent")
	fmt.Fprintln(a.out, "  loop ls      Show loop name and limits")
	fmt.Fprintln(a.out, "  guard ls     Show configured guardrails (planned)")
}

func (a *App) runAgent(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	filePath := flags.String("f", "", "Agentfile path")
	nameOverride := flags.String("name", "", "override agent name")
	dryRun := flags.Bool("dry-run", false, "parse and validate without starting the agent")
	autoRemove := flags.Bool("rm", false, "remove recorded agent state after stop")
	workDir := flags.String("workdir", ".", "agent working directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	config, err := a.configForRun(*filePath, flags.Args())
	if err != nil {
		return err
	}
	if *nameOverride != "" {
		config.Name = *nameOverride
	}

	id := instanceID(config.Name, a.now())
	logPath, tracePath, configPath, err := a.paths(id)
	if err != nil {
		return err
	}

	a.injectCredentials(&config)
	defaultExec(&config, configPath)

	if err := a.validator.Validate(config); err != nil {
		return err
	}
	if *dryRun {
		return writeJSON(a.out, config)
	}

	if err := writeConfigFile(configPath, config); err != nil {
		return err
	}

	process, err := a.driver.Start(ctx, config, driver.StartOptions{LogPath: logPath, WorkDir: *workDir})
	if err != nil {
		return err
	}

	now := a.now().UTC()
	instance := store.Instance{
		ID:         id,
		Name:       config.Name,
		Image:      config.Image,
		Type:       config.Type,
		Status:     string(driver.StatusRunning),
		PID:        process.PID,
		Config:     config,
		LogPath:    logPath,
		TracePath:  tracePath,
		ConfigPath: configPath,
		WorkDir:    *workDir,
		AutoRemove: *autoRemove,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := a.repo.Save(instance); err != nil {
		return err
	}
	if err := a.writeTrace(tracePath, trace.Event{
		Time:   now,
		Kind:   trace.KindRun,
		Agent:  id,
		Detail: fmt.Sprintf("pid=%d", process.PID),
		Fields: map[string]string{"workdir": *workDir},
	}); err != nil {
		return err
	}

	fmt.Fprintf(a.out, "%s\n", id)
	return nil
}

func (a *App) configForRun(filePath string, args []string) (agent.Config, error) {
	if len(args) > 1 {
		return agent.Config{}, fmt.Errorf("run expects at most one image")
	}
	if filePath != "" && len(args) == 1 {
		return agent.Config{}, fmt.Errorf("run accepts either -f Agentfile or IMAGE, not both")
	}

	if len(args) == 1 {
		return a.images.MustConfig(normalizeImageRef(args[0]))
	}

	if filePath == "" {
		filePath = "Agentfile"
	}
	return a.loadConfig(filePath)
}

func normalizeImageRef(value string) string {
	if catalog.IsImageRef(value) {
		return value
	}
	return value + ":latest"
}

type listOptions struct {
	all   bool
	quiet bool
}

func parseListOptions(command string, args []string, errOut io.Writer) (listOptions, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(errOut)
	all := flags.Bool("a", false, "show all agents")
	quiet := flags.Bool("q", false, "only show IDs")
	allQuiet := flags.Bool("aq", false, "show all agent IDs")
	quietAll := flags.Bool("qa", false, "show all agent IDs")
	if err := flags.Parse(args); err != nil {
		return listOptions{}, err
	}
	if flags.NArg() != 0 {
		return listOptions{}, fmt.Errorf("%s does not accept positional arguments", command)
	}
	options := listOptions{all: *all, quiet: *quiet}
	if *allQuiet || *quietAll {
		options.all = true
		options.quiet = true
	}
	return options, nil
}

func (a *App) listAgents(ctx context.Context, args []string) error {
	options, err := parseListOptions("ps", args, a.errOut)
	if err != nil {
		return err
	}

	instances, err := a.repo.List()
	if err != nil {
		return err
	}

	if !options.quiet {
		fmt.Fprintf(a.out, "%-24s %-22s %-14s %-12s %-8s %s\n", "AGENT ID", "IMAGE", "ROLE", "STATUS", "PID", "SKILLS")
	}
	for _, instance := range instances {
		status, err := a.instanceStatus(ctx, instance)
		if err != nil {
			return err
		}
		if !options.all && status != string(driver.StatusRunning) {
			continue
		}
		if options.quiet {
			fmt.Fprintf(a.out, "%s\n", instance.ID)
			continue
		}
		fmt.Fprintf(a.out, "%-24s %-22s %-14s %-12s %-8d %s\n", instance.ID, displayValue(instance.Image), instance.Type, status, instance.PID, skillList(instance.Config))
	}
	return nil
}

func (a *App) instanceStatus(ctx context.Context, instance store.Instance) (string, error) {
	if instance.PID <= 0 {
		return string(driver.StatusStopped), nil
	}
	currentStatus, err := a.driver.Status(ctx, driver.Process{PID: instance.PID})
	if err != nil {
		return "", err
	}
	return string(currentStatus), nil
}

func (a *App) logs(args []string) error {
	flags := flag.NewFlagSet("logs", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	level := flags.String("level", "", "minimum log level (debug|info|warn|error)")
	jsonOutput := flags.Bool("json", false, "emit raw JSON-Lines log records")
	if err := flags.Parse(args); err != nil {
		return err
	}
	id, err := requiredID("logs", flags.Args())
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	if instance.LogPath == "" {
		return fmt.Errorf("agent %s has no log path", id)
	}
	min, err := logging.ParseLevel(*level)
	if err != nil {
		return err
	}
	return logging.FilterFile(a.out, instance.LogPath, min, *jsonOutput)
}

func (a *App) stopAgent(ctx context.Context, args []string) error {
	id, err := requiredID("stop", args)
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}

	if err := a.driver.Stop(ctx, driver.Process{PID: instance.PID}); err != nil {
		return err
	}
	now := a.now().UTC()
	instance.Status = string(driver.StatusStopped)
	instance.UpdatedAt = now
	if err := a.writeTrace(instance.TracePath, trace.Event{
		Time:   now,
		Kind:   trace.KindStop,
		Agent:  instance.ID,
		Detail: fmt.Sprintf("pid=%d", instance.PID),
	}); err != nil {
		return err
	}
	if instance.AutoRemove {
		return a.deleteInstance(instance)
	}
	if err := a.repo.Save(instance); err != nil {
		return err
	}
	return nil
}

func (a *App) startAgent(ctx context.Context, args []string) error {
	id, err := requiredID("start", args)
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	return a.startExisting(ctx, instance)
}

func (a *App) restartAgent(ctx context.Context, args []string) error {
	id, err := requiredID("restart", args)
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	if instance.PID > 0 {
		if err := a.driver.Stop(ctx, driver.Process{PID: instance.PID}); err != nil {
			return err
		}
	}
	return a.startExisting(ctx, instance)
}

func (a *App) startExisting(ctx context.Context, instance store.Instance) error {
	process, err := a.driver.Start(ctx, instance.Config, driver.StartOptions{LogPath: instance.LogPath, WorkDir: instance.WorkDir})
	if err != nil {
		return err
	}

	now := a.now().UTC()
	instance.PID = process.PID
	instance.Status = string(driver.StatusRunning)
	instance.UpdatedAt = now
	if err := a.repo.Save(instance); err != nil {
		return err
	}
	if err := a.writeTrace(instance.TracePath, trace.Event{
		Time:   now,
		Kind:   trace.KindStart,
		Agent:  instance.ID,
		Detail: fmt.Sprintf("pid=%d", process.PID),
	}); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%s\n", instance.ID)
	return nil
}

func (a *App) inspect(args []string) error {
	id, err := requiredID("inspect", args)
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	return writeJSON(a.out, instance)
}

func (a *App) describe(ctx context.Context, args []string) error {
	id, err := requiredID("describe", args)
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	status, err := a.instanceStatus(ctx, instance)
	if err != nil {
		return err
	}
	return a.writeDescription(instance, status)
}

func (a *App) writeDescription(instance store.Instance, status string) error {
	lines := []string{
		"Agent:",
		fmt.Sprintf("  ID: %s", instance.ID),
		fmt.Sprintf("  Name: %s", displayValue(instance.Name)),
		fmt.Sprintf("  Image: %s", displayValue(instance.Image)),
		fmt.Sprintf("  Role: %s", displayValue(instance.Type)),
		fmt.Sprintf("  Status: %s", displayValue(status)),
		fmt.Sprintf("  PID: %s", pidValue(instance.PID)),
		fmt.Sprintf("  Auto Remove: %t", instance.AutoRemove),
		fmt.Sprintf("  Workdir: %s", displayValue(instance.WorkDir)),
		fmt.Sprintf("  Created: %s", timeValue(instance.CreatedAt)),
		fmt.Sprintf("  Updated: %s", timeValue(instance.UpdatedAt)),
		"",
		"Model:",
		fmt.Sprintf("  Provider: %s", displayValue(instance.Config.Model.Provider)),
		fmt.Sprintf("  Name: %s", displayValue(instance.Config.Model.Name)),
		fmt.Sprintf("  Base URL: %s", displayValue(instance.Config.Model.BaseURL)),
		fmt.Sprintf("  Auth: %s", displayValue(instance.Config.Model.Auth)),
		fmt.Sprintf("  API Key Env: %s", displayValue(instance.Config.Model.APIKeyEnv)),
		fmt.Sprintf("  Timeout Sec: %d", instance.Config.Model.TimeoutSec),
		"",
		"Loop:",
		fmt.Sprintf("  Name: %s", displayValue(instance.Config.Loop.Name)),
		fmt.Sprintf("  Max Steps: %d", instance.Config.Loop.MaxSteps),
		fmt.Sprintf("  Max Tokens: %d", instance.Config.Loop.MaxTokens),
		fmt.Sprintf("  Tool Selection: %s", displayValue(instance.Config.Loop.ToolSelection)),
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(a.out, line); err != nil {
			return err
		}
	}

	if err := writeNamedList(a.out, "Skills", skillSources(instance.Config)); err != nil {
		return err
	}
	if err := writeMCPList(a.out, instance.Config.MCPServers); err != nil {
		return err
	}
	if err := writeRAGList(a.out, "Vector RAG", instance.Config.VectorStores); err != nil {
		return err
	}
	if err := writeRAGList(a.out, "Graph RAG", instance.Config.GraphStores); err != nil {
		return err
	}
	if err := writeMemoryList(a.out, instance.Config.Memories); err != nil {
		return err
	}
	if err := writeEndpointList(a.out, instance.Config.Endpoints); err != nil {
		return err
	}
	return writeMap(a.out, "Labels", instance.Config.Labels)
}

func (a *App) listSkills(args []string) error {
	dirs := args
	if len(dirs) == 0 {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dirs = []string{"./skills", filepath.Join(homeDir, ".agentctl", "skills")}
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), ".md") {
				fmt.Fprintf(a.out, "%s\n", filepath.Join(dir, entry.Name()))
			}
		}
	}
	return nil
}

func (a *App) agents(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "ls" {
		if len(args) == 0 {
			return a.listAgents(ctx, nil)
		}
		return a.listAgents(ctx, args[1:])
	}
	if args[0] == "rm" {
		return a.removeAgents(ctx, args[1:])
	}
	if args[0] == "describe" {
		return a.describe(ctx, args[1:])
	}
	return fmt.Errorf("unknown agents command %q", args[0])
}

func (a *App) modelsCommand(args []string) error {
	if len(args) == 0 {
		return a.modelTable()
	}
	switch args[0] {
	case "ls":
		if len(args) > 1 {
			return fmt.Errorf("model ls does not accept positional arguments")
		}
		return a.modelTable()
	case "auth":
		// `model auth ls` lists every logged-in provider regardless of which
		// catalog row it came from.
		if len(args) >= 2 && args[1] == "ls" {
			return a.modelAuthList()
		}
		return fmt.Errorf("usage: agentctl model auth ls")
	}
	// Anything that isn't `ls` or `auth` is treated as a provider name so we
	// can route `model anthropic auth login`, `model openai auth status`, etc.
	provider := args[0]
	if a.models.Default(provider).Ref == "" {
		return fmt.Errorf("unknown model provider %q (try: agentctl model ls)", provider)
	}
	return a.modelProvider(provider, args[1:])
}

// modelTable renders the catalog plus a "LOGGED IN" column derived from the
// credentials store so an operator can see at a glance which providers are
// ready to use.
func (a *App) modelTable() error {
	loggedIn := map[string]bool{}
	if names, err := a.credentials.List(); err == nil {
		for _, name := range names {
			loggedIn[name] = true
		}
	}

	if _, err := fmt.Fprintf(a.out, "%-20s %-8s %-12s %-16s %-18s %-32s %s\n",
		"REF", "KIND", "RUNTIME", "AUTH", "CREDENTIAL", "ENDPOINT", "LOGGED IN"); err != nil {
		return err
	}
	for _, provider := range a.models.List() {
		credential := provider.CredentialEnv
		if credential == "" {
			credential = "-"
		}
		short := strings.SplitN(provider.Ref, ":", 2)[0]
		mark := "-"
		if loggedIn[short] {
			mark = "yes"
		}
		if _, err := fmt.Fprintf(a.out, "%-20s %-8s %-12s %-16s %-18s %-32s %s\n",
			provider.Ref, provider.Kind, provider.Runtime, provider.Auth, credential, provider.Endpoint, mark); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) skills(args []string) error {
	if len(args) == 0 || args[0] == "ls" {
		if len(args) == 0 {
			return a.listSkills(nil)
		}
		return a.listSkills(args[1:])
	}
	return fmt.Errorf("unknown skills command %q", args[0])
}

func (a *App) tools(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: agentctl tool <ls|mcp|exec> [args]")
	}
	switch args[0] {
	case "ls":
		return a.listTools(args[1:])
	case "mcp":
		return a.toolMCP(ctx, args[1:])
	case "exec":
		return a.toolExec(ctx, args[1:])
	}
	return fmt.Errorf("unknown tool command %q", args[0])
}

func (a *App) listTools(args []string) error {
	id, err := requiredID("list-tools", args)
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	if len(instance.Config.MCPServers) == 0 {
		return nil
	}

	for _, server := range instance.Config.MCPServers {
		fmt.Fprintf(a.out, "%s\t%s\t%s\n", server.Name, mcpServerTransport(server), mcpServerSummary(server))
	}
	return nil
}

func (a *App) trace(args []string) error {
	flags := flag.NewFlagSet("trace", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	jsonOutput := flags.Bool("json", false, "emit raw JSON-Lines trace events")
	if err := flags.Parse(args); err != nil {
		return err
	}
	id, err := requiredID("trace", flags.Args())
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	if instance.TracePath == "" {
		return fmt.Errorf("agent %s has no trace path", id)
	}
	if *jsonOutput {
		return printFile(a.out, instance.TracePath)
	}
	return trace.CopyHumanLines(a.out, instance.TracePath)
}

func (a *App) removeAgents(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("rm", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	forceShort := flags.Bool("f", false, "force removal of running agents")
	forceLong := flags.Bool("force", false, "force removal of running agents")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ids := flags.Args()
	if len(ids) == 0 {
		return fmt.Errorf("rm expects at least one agent id")
	}
	force := *forceShort || *forceLong

	for _, id := range ids {
		instance, err := a.repo.Find(id)
		if err != nil {
			return err
		}
		status, err := a.instanceStatus(ctx, instance)
		if err != nil {
			return err
		}
		if status == string(driver.StatusRunning) {
			if !force {
				return fmt.Errorf("cannot remove running agent %s without -f", id)
			}
			if err := a.driver.Stop(ctx, driver.Process{PID: instance.PID}); err != nil {
				return err
			}
			now := a.now().UTC()
			if err := a.writeTrace(instance.TracePath, trace.Event{
				Time:   now,
				Kind:   trace.KindRemove,
				Agent:  instance.ID,
				Detail: fmt.Sprintf("forced pid=%d", instance.PID),
			}); err != nil {
				return err
			}
		}
		if err := a.deleteInstance(instance); err != nil {
			return err
		}
		fmt.Fprintf(a.out, "%s\n", id)
	}
	return nil
}

func (a *App) deleteInstance(instance store.Instance) error {
	if err := a.repo.Delete(instance.ID); err != nil {
		return err
	}
	return removeFiles(instance.LogPath, instance.TracePath, instance.ConfigPath)
}

// loadConfig delegates to ParseFile so the FROM directive can resolve
// relative parent paths against the Agentfile being loaded.
func (a *App) loadConfig(path string) (agent.Config, error) {
	return a.parser.ParseFile(path)
}

func requiredID(command string, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("%s expects exactly one agent id", command)
	}
	return args[0], nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printFile(writer io.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func runtimePaths(id string) (string, string, string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", "", "", err
	}
	baseDir := filepath.Join(cacheDir, "agentctl")
	return filepath.Join(baseDir, "logs", id+".log"),
		filepath.Join(baseDir, "traces", id+".trace"),
		filepath.Join(baseDir, "configs", id+".json"),
		nil
}

var unsafeIDChars = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func instanceID(name string, now time.Time) string {
	clean := strings.Trim(unsafeIDChars.ReplaceAllString(name, "-"), "-")
	if clean == "" {
		clean = "agent"
	}
	return fmt.Sprintf("%s-%d", clean, now.UTC().UnixNano())
}

func skillList(config agent.Config) string {
	if len(config.Skills) == 0 {
		return "-"
	}
	values := make([]string, 0, len(config.Skills))
	for _, skill := range config.Skills {
		values = append(values, skillDisplay(skill))
	}
	return strings.Join(values, ",")
}

func skillSources(config agent.Config) []string {
	values := make([]string, 0, len(config.Skills))
	for _, skill := range config.Skills {
		values = append(values, skillDisplay(skill))
	}
	return values
}

func skillDisplay(skill agent.Skill) string {
	if skill.Path != "" {
		return skill.Path
	}
	if skill.Name != "" {
		return skill.Name
	}
	return skill.ID
}

func displayValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func pidValue(pid int) string {
	if pid <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", pid)
}

func timeValue(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func writeNamedList(writer io.Writer, title string, values []string) error {
	if _, err := fmt.Fprintf(writer, "\n%s:\n", title); err != nil {
		return err
	}
	if len(values) == 0 {
		_, err := fmt.Fprintln(writer, "  -")
		return err
	}
	for _, value := range values {
		if _, err := fmt.Fprintf(writer, "  - %s\n", value); err != nil {
			return err
		}
	}
	return nil
}

func writeMCPList(writer io.Writer, servers []agent.MCPServer) error {
	if _, err := fmt.Fprintln(writer, "\nMCP Tools:"); err != nil {
		return err
	}
	if len(servers) == 0 {
		_, err := fmt.Fprintln(writer, "  -")
		return err
	}
	for _, server := range servers {
		if _, err := fmt.Fprintf(writer, "  - %s [%s] %s\n", server.Name, mcpServerTransport(server), mcpServerSummary(server)); err != nil {
			return err
		}
	}
	return nil
}

func mcpServerTransport(server agent.MCPServer) string {
	if server.Command != "" {
		return mcp.TransportStdio
	}
	return mcp.TransportHTTP
}

func mcpServerSummary(server agent.MCPServer) string {
	if server.Command != "" {
		if len(server.Args) == 0 {
			return server.Command
		}
		return server.Command + " " + strings.Join(server.Args, " ")
	}
	if server.BasePath == "" {
		return server.URL
	}
	return strings.TrimRight(server.URL, "/") + "/" + strings.TrimLeft(server.BasePath, "/")
}

func writeRAGList(writer io.Writer, title string, sources []agent.RAGSource) error {
	if _, err := fmt.Fprintf(writer, "\n%s:\n", title); err != nil {
		return err
	}
	if len(sources) == 0 {
		_, err := fmt.Fprintln(writer, "  -")
		return err
	}
	for _, source := range sources {
		index := source.Index
		if index == "" {
			index = "-"
		}
		if _, err := fmt.Fprintf(writer, "  - %s type=%s provider=%s index=%s url=%s\n", source.Name, source.Type, source.Provider, index, source.URL); err != nil {
			return err
		}
	}
	return nil
}

func writeMemoryList(writer io.Writer, memories []agent.Memory) error {
	if _, err := fmt.Fprintln(writer, "\nMemory:"); err != nil {
		return err
	}
	if len(memories) == 0 {
		_, err := fmt.Fprintln(writer, "  -")
		return err
	}
	for _, memory := range memories {
		if _, err := fmt.Fprintf(writer, "  - %s type=%s provider=%s bucket=%s url=%s limit=%d ttl_sec=%d\n", memory.Name, memory.Type, displayValue(memory.Provider), displayValue(memory.Bucket), displayValue(memory.URL), memory.Limit, memory.TTLSec); err != nil {
			return err
		}
	}
	return nil
}

func writeEndpointList(writer io.Writer, endpoints []agent.Endpoint) error {
	if _, err := fmt.Fprintln(writer, "\nEndpoints:"); err != nil {
		return err
	}
	if len(endpoints) == 0 {
		_, err := fmt.Fprintln(writer, "  -")
		return err
	}
	for _, endpoint := range endpoints {
		if _, err := fmt.Fprintf(writer, "  - %s -> %s\n", endpoint.Name, agent.EndpointURL(endpoint)); err != nil {
			return err
		}
	}
	return nil
}

func writeMap(writer io.Writer, title string, values map[string]string) error {
	if _, err := fmt.Fprintf(writer, "\n%s:\n", title); err != nil {
		return err
	}
	if len(values) == 0 {
		_, err := fmt.Fprintln(writer, "  -")
		return err
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := fmt.Fprintf(writer, "  - %s=%s\n", key, values[key]); err != nil {
			return err
		}
	}
	return nil
}

func removeFiles(paths ...string) error {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (a *App) writeTrace(path string, event trace.Event) error {
	if path == "" {
		return nil
	}
	return trace.NewFileWriter(path).Write(event)
}
