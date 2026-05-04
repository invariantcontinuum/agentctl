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
	"strings"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/agentfile"
	"github.com/invariantcontinuum/agentctl/internal/catalog"
	"github.com/invariantcontinuum/agentctl/internal/driver"
	"github.com/invariantcontinuum/agentctl/internal/model"
	"github.com/invariantcontinuum/agentctl/internal/store"
)

type App struct {
	out       io.Writer
	errOut    io.Writer
	parser    agentfile.Parser
	validator agent.Validator
	repo      store.Repository
	driver    driver.Driver
	images    catalog.Catalog
	models    model.Catalog
	now       func() time.Time
	paths     func(string) (string, string, error)
}

func New(out io.Writer, errOut io.Writer, repo store.Repository, runtimeDriver driver.Driver) *App {
	return &App{
		out:       out,
		errOut:    errOut,
		parser:    agentfile.NewParser(),
		validator: agent.ConfigValidator{},
		repo:      repo,
		driver:    runtimeDriver,
		images:    catalog.DefaultCatalog(),
		models:    model.DefaultCatalog(),
		now:       time.Now,
		paths:     runtimePaths,
	}
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
	case "agents":
		err = a.agents(ctx, args[1:])
	case "models":
		err = a.modelsCommand(args[1:])
	case "logs":
		err = a.logs(args[1:])
	case "stop":
		err = a.stopAgent(ctx, args[1:])
	case "start":
		err = a.startAgent(ctx, args[1:])
	case "restart":
		err = a.restartAgent(ctx, args[1:])
	case "inspect":
		err = a.inspect(args[1:])
	case "list-skills":
		err = a.listSkills(args[1:])
	case "list-tools":
		err = a.listTools(args[1:])
	case "skills":
		err = a.skills(args[1:])
	case "tools":
		err = a.tools(args[1:])
	case "trace":
		err = a.trace(args[1:])
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
	fmt.Fprintln(a.out, "Commands:")
	fmt.Fprintln(a.out, "  run          Start an agent from an Agentfile or image")
	fmt.Fprintln(a.out, "  ps           List agents")
	fmt.Fprintln(a.out, "  agents ls    List agents")
	fmt.Fprintln(a.out, "  models ls    List model provider definitions")
	fmt.Fprintln(a.out, "  logs         Print an agent log")
	fmt.Fprintln(a.out, "  stop         Stop an agent process")
	fmt.Fprintln(a.out, "  start        Start a stopped agent")
	fmt.Fprintln(a.out, "  restart      Restart an agent")
	fmt.Fprintln(a.out, "  inspect      Print agent configuration as JSON")
	fmt.Fprintln(a.out, "  skills ls    List skills in one or more directories")
	fmt.Fprintln(a.out, "  tools ls     List configured MCP servers for an agent")
	fmt.Fprintln(a.out, "  trace        Print local lifecycle trace events")
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
	if err := a.validator.Validate(config); err != nil {
		return err
	}
	if *dryRun {
		return writeJSON(a.out, config)
	}

	id := instanceID(config.Name, a.now())
	logPath, tracePath, err := a.paths(id)
	if err != nil {
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
		WorkDir:    *workDir,
		AutoRemove: *autoRemove,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := a.repo.Save(instance); err != nil {
		return err
	}
	if err := appendTrace(tracePath, now, "run", fmt.Sprintf("pid=%d", process.PID)); err != nil {
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
	id, err := requiredID("logs", args)
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
	return printFile(a.out, instance.LogPath)
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
	if err := appendTrace(instance.TracePath, now, "stop", fmt.Sprintf("pid=%d", instance.PID)); err != nil {
		return err
	}
	if instance.AutoRemove {
		return a.repo.Delete(instance.ID)
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
	if err := appendTrace(instance.TracePath, now, "start", fmt.Sprintf("pid=%d", process.PID)); err != nil {
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
	return fmt.Errorf("unknown agents command %q", args[0])
}

func (a *App) modelsCommand(args []string) error {
	if len(args) == 0 || args[0] == "ls" {
		if len(args) > 1 {
			return fmt.Errorf("models ls does not accept positional arguments")
		}
		return a.models.WriteTable(a.out)
	}
	return fmt.Errorf("unknown models command %q", args[0])
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

func (a *App) tools(args []string) error {
	if len(args) == 0 || args[0] != "ls" {
		return fmt.Errorf("usage: agentctl tools ls <agent-id>")
	}
	return a.listTools(args[1:])
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
		fmt.Fprintf(a.out, "%s\t%s\n", server.Name, server.URL)
	}
	return nil
}

func (a *App) trace(args []string) error {
	id, err := requiredID("trace", args)
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
	return printFile(a.out, instance.TracePath)
}

func (a *App) loadConfig(path string) (agent.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return agent.Config{}, err
	}
	defer file.Close()
	return a.parser.Parse(file)
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

func runtimePaths(id string) (string, string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", "", err
	}
	baseDir := filepath.Join(cacheDir, "agentctl")
	return filepath.Join(baseDir, "logs", id+".log"), filepath.Join(baseDir, "traces", id+".trace"), nil
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
		values = append(values, skill.Source)
	}
	return strings.Join(values, ",")
}

func displayValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func appendTrace(path string, when time.Time, event string, detail string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintf(file, "%s %s %s\n", when.UTC().Format(time.RFC3339Nano), event, detail)
	return err
}
