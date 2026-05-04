package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
		if err := a.waitForServiceHealth(ctx, document, service); err != nil {
			return fmt.Errorf("compose service %q: %w", service.Name, err)
		}
	}
	return nil
}

func (a *App) startComposeService(ctx context.Context, document compose.Document, service compose.Service, composeDir string) error {
	manifestPath := service.File
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(composeDir, manifestPath)
	}
	config, err := a.loadConfig(manifestPath)
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

	id := instanceID(service.Name, a.now())
	logPath, tracePath, configPath, err := a.paths(id)
	if err != nil {
		return err
	}
	a.injectCredentials(&config)
	defaultExec(&config, configPath)

	if err := a.validator.Validate(config); err != nil {
		return err
	}
	if err := writeConfigFile(configPath, config); err != nil {
		return err
	}

	process, err := a.driver.Start(ctx, config, driver.StartOptions{LogPath: logPath, WorkDir: composeDir})
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
		WorkDir:    composeDir,
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

// waitForServiceHealth polls the runtime contract's /health endpoint for the
// freshly started service so dependent services (DEPENDS_ON) only run once
// the prerequisite is actually serving requests. Services that don't declare
// an `ENDPOINT http <url>` are skipped — there is nothing to probe.
func (a *App) waitForServiceHealth(ctx context.Context, document compose.Document, service compose.Service) error {
	instances, err := a.repo.List()
	if err != nil {
		return err
	}
	var instance store.Instance
	found := false
	for _, candidate := range instances {
		if candidate.Config.Labels[composeProjectLabel] == document.Name &&
			candidate.Config.Labels[composeServiceLabel] == service.Name {
			instance = candidate
			found = true
		}
	}
	if !found {
		return nil
	}
	url := httpEndpointURL(instance.Config)
	if url == "" {
		return nil
	}
	probe := a.healthProbeFor(instance.ID)
	overall, cancelOverall := context.WithTimeout(ctx, 20*time.Second)
	defer cancelOverall()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		probeCtx, cancel := context.WithTimeout(overall, 2*time.Second)
		report, err := probe.Run(probeCtx, url)
		cancel()
		if err == nil && len(report.Probes) > 0 && report.Probes[0].OK {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-overall.Done():
			return fmt.Errorf("service %q failed health probe at %s", service.Name, url)
		case <-ticker.C:
		}
	}
}

func httpEndpointURL(config agent.Config) string {
	for _, endpoint := range config.Endpoints {
		if strings.EqualFold(endpoint.Name, "http") {
			return agent.EndpointURL(endpoint)
		}
	}
	return ""
}

// Compile-time check that agent.Config is the type we use here.
var _ = agent.Config{}
