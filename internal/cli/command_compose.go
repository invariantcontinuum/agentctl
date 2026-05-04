package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/compose"
	"github.com/invariantcontinuum/agentctl/internal/driver"
	"github.com/invariantcontinuum/agentctl/internal/store"
	"github.com/invariantcontinuum/agentctl/internal/trace"
)

const composeServiceLabel = "agentctl.compose.service"
const composeProjectLabel = "agentctl.compose.project"

func (a *App) compose(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentctl compose <up|down|ls|ps> [-f path]")
	}
	switch args[0] {
	case "up":
		return a.composeUp(ctx, args[1:])
	case "down":
		return a.composeDown(ctx, args[1:])
	case "ls":
		return a.composeLs(args[1:])
	case "ps":
		return a.composePs(ctx, args[1:])
	}
	return fmt.Errorf("unknown compose command %q", args[0])
}

func (a *App) composeUp(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("compose up", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	filePath := flags.String("f", "AgentCompose", "AgentCompose file path")
	dryRun := flags.Bool("dry-run", false, "validate and print plan without starting agents")
	if err := flags.Parse(args); err != nil {
		return err
	}

	document, services, err := a.loadComposePlan(*filePath)
	if err != nil {
		return err
	}

	if *dryRun {
		for _, service := range services {
			fmt.Fprintf(a.out, "%s\t%s\n", service.Name, service.File)
		}
		return nil
	}

	composeDir := filepath.Dir(*filePath)
	for _, service := range services {
		if err := a.startComposeService(ctx, document, service, composeDir); err != nil {
			return fmt.Errorf("compose service %q: %w", service.Name, err)
		}
	}
	return nil
}

func (a *App) startComposeService(ctx context.Context, document compose.Document, service compose.Service, composeDir string) error {
	configPath := service.File
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(composeDir, configPath)
	}
	config, err := a.loadConfig(configPath)
	if err != nil {
		return err
	}
	if config.Name == "" {
		config.Name = service.Name
	}
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[composeProjectLabel] = document.Name
	config.Labels[composeServiceLabel] = service.Name
	if err := a.validator.Validate(config); err != nil {
		return err
	}

	id := instanceID(service.Name, a.now())
	logPath, tracePath, err := a.paths(id)
	if err != nil {
		return err
	}

	process, err := a.driver.Start(ctx, config, driver.StartOptions{LogPath: logPath, WorkDir: composeDir})
	if err != nil {
		return err
	}

	now := a.now().UTC()
	instance := store.Instance{
		ID:        id,
		Name:      config.Name,
		Image:     config.Image,
		Type:      config.Type,
		Status:    string(driver.StatusRunning),
		PID:       process.PID,
		Config:    config,
		LogPath:   logPath,
		TracePath: tracePath,
		WorkDir:   composeDir,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := a.repo.Save(instance); err != nil {
		return err
	}
	if err := a.writeTrace(tracePath, trace.Event{
		Time:   now,
		Kind:   trace.KindRun,
		Agent:  id,
		Detail: fmt.Sprintf("compose=%s service=%s pid=%d", document.Name, service.Name, process.PID),
		Fields: map[string]string{"compose": document.Name, "service": service.Name},
	}); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%s\t%s\n", service.Name, id)
	return nil
}

func (a *App) composeDown(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("compose down", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	filePath := flags.String("f", "AgentCompose", "AgentCompose file path")
	if err := flags.Parse(args); err != nil {
		return err
	}

	document, _, err := a.loadComposePlan(*filePath)
	if err != nil {
		return err
	}

	instances, err := a.repo.List()
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if instance.Config.Labels[composeProjectLabel] != document.Name {
			continue
		}
		status, err := a.instanceStatus(ctx, instance)
		if err != nil {
			return err
		}
		if status == string(driver.StatusRunning) {
			if err := a.driver.Stop(ctx, driver.Process{PID: instance.PID}); err != nil {
				return err
			}
			now := a.now().UTC()
			if err := a.writeTrace(instance.TracePath, trace.Event{
				Time:   now,
				Kind:   trace.KindStop,
				Agent:  instance.ID,
				Detail: fmt.Sprintf("compose=%s pid=%d", document.Name, instance.PID),
			}); err != nil {
				return err
			}
		}
		if err := a.deleteInstance(instance); err != nil {
			return err
		}
		fmt.Fprintf(a.out, "%s\n", instance.ID)
	}
	return nil
}

func (a *App) composeLs(args []string) error {
	flags := flag.NewFlagSet("compose ls", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	filePath := flags.String("f", "AgentCompose", "AgentCompose file path")
	if err := flags.Parse(args); err != nil {
		return err
	}

	document, services, err := a.loadComposePlan(*filePath)
	if err != nil {
		return err
	}

	fmt.Fprintf(a.out, "PROJECT %s\n", document.Name)
	fmt.Fprintf(a.out, "%-16s %-32s %s\n", "SERVICE", "FILE", "DEPENDS_ON")
	for _, service := range services {
		fmt.Fprintf(a.out, "%-16s %-32s %s\n", service.Name, service.File, depsValue(service.DependsOn))
	}
	return nil
}

func (a *App) composePs(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("compose ps", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	filePath := flags.String("f", "AgentCompose", "AgentCompose file path")
	if err := flags.Parse(args); err != nil {
		return err
	}

	document, _, err := a.loadComposePlan(*filePath)
	if err != nil {
		return err
	}

	instances, err := a.repo.List()
	if err != nil {
		return err
	}

	matching := filterByLabel(instances, composeProjectLabel, document.Name)
	sort.Slice(matching, func(left, right int) bool {
		return matching[left].Config.Labels[composeServiceLabel] < matching[right].Config.Labels[composeServiceLabel]
	})

	fmt.Fprintf(a.out, "%-16s %-24s %-12s %s\n", "SERVICE", "AGENT ID", "STATUS", "PID")
	for _, instance := range matching {
		status, err := a.instanceStatus(ctx, instance)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "%-16s %-24s %-12s %s\n",
			instance.Config.Labels[composeServiceLabel],
			instance.ID,
			status,
			pidValue(instance.PID),
		)
	}
	return nil
}

func (a *App) loadComposePlan(filePath string) (compose.Document, []compose.Service, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return compose.Document{}, nil, err
	}
	defer file.Close()
	document, err := a.composeParser.Parse(file)
	if err != nil {
		return compose.Document{}, nil, err
	}
	services, err := document.Plan()
	if err != nil {
		return compose.Document{}, nil, err
	}
	return document, services, nil
}

func filterByLabel(instances []store.Instance, key string, value string) []store.Instance {
	matched := make([]store.Instance, 0)
	for _, instance := range instances {
		if instance.Config.Labels[key] == value {
			matched = append(matched, instance)
		}
	}
	return matched
}

func depsValue(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

// Compile-time check that agent.Config is the type we use here.
var _ = agent.Config{}
