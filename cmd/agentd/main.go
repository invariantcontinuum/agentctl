// Command agentd is the agent runtime binary that hosts a single agent
// instance behind the runtime contract documented in docs/agentfile.md.
//
// Usage:
//
//	agentd --config <path> [--addr host:port] [--log-level debug|info|warn|error]
//
// The CLI's `agentctl run` command writes the parsed agent.Config to a JSON
// file and execs agentd with --config pointing at it. agentd then exposes:
//
//	GET  /health
//	GET  /status
//	GET  /tasks
//	POST /tasks
//	GET  /tasks/{id}
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/logging"
	"github.com/invariantcontinuum/agentctl/internal/runtime"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "agentd: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("agentd", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to JSON-encoded agent.Config")
	addr := flags.String("addr", "", "HTTP listen address (default derives from agent ENDPOINT http)")
	logLevel := flags.String("log-level", "info", "minimum log level (debug|info|warn|error)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return fmt.Errorf("--config is required")
	}

	config, err := loadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	level, err := logging.ParseLevel(*logLevel)
	if err != nil {
		return err
	}
	logger := logging.New(stdout, level)

	rt := runtime.New(config, runtime.Options{
		Address: *addr,
		Logger:  logger,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("agentd starting",
		"config", *configPath,
		"agent", config.Name,
		"addr", rt.Address(),
	)
	return rt.Start(ctx)
}

func loadConfig(path string) (agent.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return agent.Config{}, err
	}
	defer file.Close()
	var config agent.Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return agent.Config{}, err
	}
	return config, nil
}
