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
	"github.com/invariantcontinuum/agentctl/internal/driver"
	"github.com/invariantcontinuum/agentctl/internal/store"
)

type App struct {
	out       io.Writer
	errOut    io.Writer
	parser    agentfile.Parser
	validator agent.Validator
	repo      store.Repository
	driver    driver.Driver
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
		err = a.listAgents()
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
	fmt.Fprintln(a.out, "  run          Start an agent from an Agentfile")
	fmt.Fprintln(a.out, "  ps           List known agents")
	fmt.Fprintln(a.out, "  logs         Print an agent log")
	fmt.Fprintln(a.out, "  stop         Stop an agent process")
	fmt.Fprintln(a.out, "  start        Start a stopped agent")
	fmt.Fprintln(a.out, "  restart      Restart an agent")
	fmt.Fprintln(a.out, "  inspect      Print agent configuration as JSON")
	fmt.Fprintln(a.out, "  list-skills  List skills in one or more directories")
	fmt.Fprintln(a.out, "  list-tools   List configured MCP servers for an agent")
	fmt.Fprintln(a.out, "  trace        Print local lifecycle trace events")
}

func (a *App) runAgent(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	filePath := flags.String("f", "Agentfile", "Agentfile path")
	nameOverride := flags.String("name", "", "override agent name")
	dryRun := flags.Bool("dry-run", false, "parse and validate without starting the agent")
	workDir := flags.String("workdir", ".", "agent working directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	config, err := a.loadConfig(*filePath)
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
		ID:        id,
		Name:      config.Name,
		Type:      config.Type,
		Status:    string(driver.StatusRunning),
		PID:       process.PID,
		Config:    config,
		LogPath:   logPath,
		TracePath: tracePath,
		WorkDir:   *workDir,
		CreatedAt: now,
		UpdatedAt: now,
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

func (a *App) listAgents() error {
	instances, err := a.repo.List()
	if err != nil {
		return err
	}

	fmt.Fprintf(a.out, "%-24s %-16s %-12s %-8s %s\n", "ID", "TYPE", "STATUS", "PID", "SKILLS")
	for _, instance := range instances {
		status := instance.Status
		if instance.PID > 0 {
			currentStatus, err := a.driver.Status(context.Background(), driver.Process{PID: instance.PID})
			if err != nil {
				return err
			}
			status = string(currentStatus)
		}
		fmt.Fprintf(a.out, "%-24s %-16s %-12s %-8d %s\n", instance.ID, instance.Type, status, instance.PID, skillList(instance.Config))
	}
	return nil
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
	if err := a.repo.Save(instance); err != nil {
		return err
	}
	return appendTrace(instance.TracePath, now, "stop", fmt.Sprintf("pid=%d", instance.PID))
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
