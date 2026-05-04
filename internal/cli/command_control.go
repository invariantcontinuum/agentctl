package cli

import (
	"context"
	"fmt"

	"github.com/invariantcontinuum/agentctl/internal/driver"
)

func (a *App) loop(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentctl loop <ls|ps|trace> [agent-id]")
	}
	switch args[0] {
	case "ls":
		return a.loopList(ctx, args[1:])
	case "ps":
		return a.loopPs(ctx, args[1:])
	case "trace":
		return a.loopTrace(args[1:])
	}
	return fmt.Errorf("unknown loop command %q", args[0])
}

// loopList shows the loop strategy and step ceiling per recorded agent so an
// operator can see at a glance which loops are running.
func (a *App) loopList(ctx context.Context, _ []string) error {
	instances, err := a.repo.List()
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%-24s %-12s %-10s %s\n", "AGENT ID", "STRATEGY", "MAX_STEPS", "STATUS")
	for _, instance := range instances {
		status, err := a.instanceStatus(ctx, instance)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "%-24s %-12s %-10d %s\n", instance.ID, displayValue(instance.Config.Loop.Strategy), instance.Config.Loop.MaxSteps, status)
	}
	return nil
}

func (a *App) loopPs(ctx context.Context, args []string) error {
	id, err := requiredID("loop ps", args)
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
	fmt.Fprintf(a.out, "Agent: %s\nStrategy: %s\nMax Steps: %d\nStatus: %s\nPID: %s\n",
		instance.ID,
		displayValue(instance.Config.Loop.Strategy),
		instance.Config.Loop.MaxSteps,
		status,
		pidValue(instance.PID),
	)
	if status != string(driver.StatusRunning) {
		return nil
	}
	return nil
}

// loopTrace is an alias for the trace command but scoped to loop-related
// kinds. It currently reuses the shared trace file and lets callers grep.
func (a *App) loopTrace(args []string) error {
	id, err := requiredID("loop trace", args)
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

func (a *App) guard(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentctl guard <ls> <agent-id>")
	}
	if args[0] != "ls" {
		return fmt.Errorf("unknown guard command %q", args[0])
	}
	id, err := requiredID("guard ls", args[1:])
	if err != nil {
		return err
	}
	if _, err := a.repo.Find(id); err != nil {
		return err
	}
	// Guardrails are documented as a target capability in
	// docs/concepts/agentic-visibility.md; surface a deterministic placeholder
	// instead of pretending to enumerate them.
	fmt.Fprintln(a.out, "no guardrails configured (Agentfile GUARD directive is reserved for a future release)")
	return nil
}
