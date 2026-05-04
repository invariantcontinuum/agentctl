package cli

import (
	"fmt"
	"strings"
)

func (a *App) memory(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentctl memory <ls|short|long|dump|recall> <agent-id> [...]")
	}
	switch args[0] {
	case "ls":
		return a.memoryList(args[1:], "")
	case "short":
		if len(args) >= 2 && args[1] == "ls" {
			return a.memoryList(args[2:], "short")
		}
	case "long":
		if len(args) >= 2 && args[1] == "ls" {
			return a.memoryList(args[2:], "long")
		}
	case "dump":
		return a.memoryDump(args[1:])
	case "recall":
		return a.memoryRecall(args[1:])
	}
	return fmt.Errorf("unknown memory command %v", args)
}

func (a *App) memoryList(args []string, kind string) error {
	id, err := requiredID("memory ls", args)
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}

	fmt.Fprintf(a.out, "%-12s %-12s %-12s %-16s %s\n", "NAME", "TYPE", "PROVIDER", "BUCKET", "URL")
	wrote := 0
	for _, memory := range instance.Config.Memories {
		if kind != "" && !strings.EqualFold(memory.Type, kind) {
			continue
		}
		fmt.Fprintf(a.out, "%-12s %-12s %-12s %-16s %s\n", memory.Name, memory.Type, memory.Provider, displayValue(memory.Bucket), displayValue(memory.URL))
		wrote++
	}
	if wrote == 0 {
		fmt.Fprintln(a.out, "-")
	}
	return nil
}

// memoryDump emits the recorded Memory configuration as JSON. The runtime side
// (live short/long memory state from the agent process) lives behind the agent
// runtime contract documented in docs/agentfile.md and is not yet implemented.
func (a *App) memoryDump(args []string) error {
	id, err := requiredID("memory dump", args)
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}
	return writeJSON(a.out, instance.Config.Memories)
}

// memoryRecall is a placeholder for runtime recall over the agent's memory
// store. Until the runtime side ships, surface the configured memory binding
// matching the key so the CLI fails with a clear message rather than silently.
func (a *App) memoryRecall(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: agentctl memory recall <agent-id> <key>")
	}
	instance, err := a.repo.Find(args[0])
	if err != nil {
		return err
	}
	key := args[1]
	for _, memory := range instance.Config.Memories {
		if memory.Name == key {
			source := memory.URL
			if source == "" {
				source = memory.Bucket
			}
			fmt.Fprintf(a.out, "%s\t%s\t%s\n", memory.Name, memory.Type, source)
			return nil
		}
	}
	return fmt.Errorf("agent %s has no memory binding %q (runtime recall not yet implemented)", args[0], key)
}
