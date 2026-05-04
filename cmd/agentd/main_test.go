package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

func writeConfigFile(t *testing.T, dir string, config agent.Config) string {
	t.Helper()
	path := filepath.Join(dir, "config.json")
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestRunRequiresConfig(t *testing.T) {
	var stderr bytes.Buffer
	if err := run(nil, &bytes.Buffer{}, &stderr); err == nil {
		t.Fatalf("expected error when --config is missing")
	}
}

func TestRunRejectsBadLevel(t *testing.T) {
	dir := t.TempDir()
	config := agent.Config{Name: "x", Type: "planner", Loop: agent.Loop{Name: "react", MaxSteps: 1}, Exec: []string{"unused"}}
	path := writeConfigFile(t, dir, config)

	if err := run([]string{"--config", path, "--log-level", "garbage"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error for bad log level")
	}
}

func TestLoadConfigReadsJSON(t *testing.T) {
	dir := t.TempDir()
	config := agent.Config{Name: "load-me", Type: "planner", Loop: agent.Loop{Name: "react", MaxSteps: 1}, Exec: []string{"unused"}}
	path := writeConfigFile(t, dir, config)

	got, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if got.Name != "load-me" {
		t.Fatalf("name = %q", got.Name)
	}
}

func TestLoadConfigErrorsOnMissingFile(t *testing.T) {
	if _, err := loadConfig(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}
