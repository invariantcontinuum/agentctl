package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevelAccepts(t *testing.T) {
	tests := []struct {
		in  string
		out Level
	}{
		{"", LevelInfo},
		{"debug", LevelDebug},
		{"INFO", LevelInfo},
		{"warning", LevelWarn},
		{"err", LevelError},
	}
	for _, test := range tests {
		got, err := ParseLevel(test.in)
		if err != nil {
			t.Fatalf("ParseLevel(%q) err = %v", test.in, err)
		}
		if got != test.out {
			t.Fatalf("ParseLevel(%q) = %v, want %v", test.in, got, test.out)
		}
	}
}

func TestParseLevelRejectsUnknown(t *testing.T) {
	if _, err := ParseLevel("trace"); err == nil {
		t.Fatal("ParseLevel returned nil error for unknown level")
	}
}

func TestLoggerEmitsAtOrAboveMinLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger := New(&buffer, LevelWarn)
	logger.Debug("debug-msg")
	logger.Info("info-msg")
	logger.Warn("warn-msg", "key", "value")
	logger.Error("error-msg")

	output := buffer.String()
	if strings.Contains(output, "debug-msg") || strings.Contains(output, "info-msg") {
		t.Fatalf("output leaked below-min lines: %s", output)
	}
	if !strings.Contains(output, `"level":"warn"`) || !strings.Contains(output, `"level":"error"`) {
		t.Fatalf("output missing warn/error lines: %s", output)
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Fatalf("output missing fields: %s", output)
	}
}

func TestFilterFileFiltersAndPrettyPrints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")
	if err := writeStringFile(path, `
{"ts":"2026-05-04T10:00:00Z","level":"debug","msg":"connecting"}
{"ts":"2026-05-04T10:00:01Z","level":"info","msg":"started"}
{"ts":"2026-05-04T10:00:02Z","level":"error","msg":"oops","fields":{"err":"boom"}}
plain text line
`); err != nil {
		t.Fatalf("writeStringFile returned error: %v", err)
	}

	var buffer bytes.Buffer
	if err := FilterFile(&buffer, path, LevelInfo, false); err != nil {
		t.Fatalf("FilterFile returned error: %v", err)
	}
	output := buffer.String()
	if strings.Contains(output, "connecting") {
		t.Fatalf("output leaked debug line: %s", output)
	}
	if !strings.Contains(output, "INFO started") {
		t.Fatalf("output missing started line: %s", output)
	}
	if !strings.Contains(output, "ERROR oops err=boom") {
		t.Fatalf("output missing error fields: %s", output)
	}
	if !strings.Contains(output, "plain text line") {
		t.Fatalf("output missing fallback raw line: %s", output)
	}
}

func TestFilterFileJSONOutputPreservesShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")
	if err := writeStringFile(path, `{"ts":"2026-05-04T10:00:00Z","level":"warn","msg":"watch"}`); err != nil {
		t.Fatalf("writeStringFile returned error: %v", err)
	}
	var buffer bytes.Buffer
	if err := FilterFile(&buffer, path, LevelInfo, true); err != nil {
		t.Fatalf("FilterFile returned error: %v", err)
	}
	if !strings.Contains(buffer.String(), `"level":"warn"`) {
		t.Fatalf("json output missing record: %s", buffer.String())
	}
}

func writeStringFile(path string, body string) error {
	return os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o600)
}
