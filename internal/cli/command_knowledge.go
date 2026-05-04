package cli

import (
	"flag"
	"fmt"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

func (a *App) rag(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentctl rag <ls|vector|graph> <agent-id>")
	}
	switch args[0] {
	case "ls":
		return a.ragList(args[1:], ragKindAll)
	case "vector":
		if len(args) >= 2 && args[1] == "ls" {
			return a.ragList(args[2:], ragKindVector)
		}
	case "graph":
		if len(args) >= 2 && args[1] == "ls" {
			return a.ragList(args[2:], ragKindGraph)
		}
	}
	return fmt.Errorf("unknown rag command %v", args)
}

type ragKind int

const (
	ragKindAll ragKind = iota
	ragKindVector
	ragKindGraph
)

func (a *App) ragList(args []string, kind ragKind) error {
	flags := flag.NewFlagSet("rag", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	if err := flags.Parse(args); err != nil {
		return err
	}
	id, err := requiredID("rag", flags.Args())
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}

	if kind == ragKindAll || kind == ragKindVector {
		if err := writeRAGTable(a.out, "VECTOR", instance.Config.VectorStores); err != nil {
			return err
		}
	}
	if kind == ragKindAll || kind == ragKindGraph {
		if err := writeRAGTable(a.out, "GRAPH", instance.Config.GraphStores); err != nil {
			return err
		}
	}
	return nil
}

func writeRAGTable(writer interface{ Write(p []byte) (int, error) }, kind string, sources []agent.RAGSource) error {
	if _, err := fmt.Fprintf(writer, "%-7s %-12s %-12s %-16s %s\n", "KIND", "NAME", "PROVIDER", "COLLECTION", "DSN"); err != nil {
		return err
	}
	if len(sources) == 0 {
		_, err := fmt.Fprintf(writer, "%-7s %s\n", kind, "-")
		return err
	}
	for _, source := range sources {
		collection := source.Collection
		if collection == "" {
			collection = "-"
		}
		if _, err := fmt.Fprintf(writer, "%-7s %-12s %-12s %-16s %s\n", kind, source.Name, source.Provider, collection, source.DSN); err != nil {
			return err
		}
	}
	return nil
}
