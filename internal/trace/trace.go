// Package trace records structured agent lifecycle and reasoning events.
//
// Events are written as one JSON object per line so a downstream reader can
// stream them with the standard library alone. The Kind taxonomy mirrors the
// agentic visibility model documented in docs/concepts/taxonomy.md: lifecycle
// events for the runtime (run, start, stop, restart, rm) and reasoning events
// for the agent loop (plan, rag, tool, memory, guard, reflection, delegation).
package trace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Kind classifies the trace event so readers can filter without parsing detail.
type Kind string

const (
	KindRun        Kind = "run"
	KindStart      Kind = "start"
	KindStop       Kind = "stop"
	KindRestart    Kind = "restart"
	KindRemove     Kind = "rm"
	KindPlan       Kind = "plan"
	KindRAG        Kind = "rag"
	KindTool       Kind = "tool"
	KindMemory     Kind = "memory"
	KindGuard      Kind = "guard"
	KindReflection Kind = "reflection"
	KindDelegation Kind = "delegation"
	KindHealth     Kind = "health"
)

// Event is a single structured trace record.
type Event struct {
	Time   time.Time         `json:"ts"`
	Kind   Kind              `json:"kind"`
	Agent  string            `json:"agent,omitempty"`
	Detail string            `json:"detail,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

// Writer appends Events to a JSON-Lines file. Concrete implementations may
// route to other transports without changing callers.
type Writer interface {
	Write(Event) error
}

// FileWriter is the default Writer that persists events on disk.
type FileWriter struct {
	path string
}

// NewFileWriter returns a Writer bound to path. The directory is created on
// the first Write call so callers can construct one before the agent runs.
func NewFileWriter(path string) *FileWriter {
	return &FileWriter{path: path}
}

// Path exposes the file location for callers that need to display it.
func (w *FileWriter) Path() string { return w.path }

// Write appends the event as a single JSON line. A zero Time is replaced with
// the current UTC clock so callers can supply only Kind+Detail at the call
// site.
func (w *FileWriter) Write(event Event) error {
	if w.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return err
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	} else {
		event.Time = event.Time.UTC()
	}

	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, err = file.Write(encoded)
	return err
}

// Read parses every line of path into Events. Empty paths and missing files
// return an empty slice so describe-style commands can call it unconditionally.
func Read(path string) ([]Event, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return nil, fmt.Errorf("trace line %d: %w", lineNumber, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// FormatLine renders an event as a single human-friendly line. Field order is
// stable so tests can assert the output verbatim.
func FormatLine(event Event) string {
	builder := strings.Builder{}
	builder.WriteString(event.Time.UTC().Format(time.RFC3339Nano))
	builder.WriteString(" ")
	builder.WriteString(string(event.Kind))
	if event.Agent != "" {
		builder.WriteString(" agent=")
		builder.WriteString(event.Agent)
	}
	if event.Detail != "" {
		builder.WriteString(" ")
		builder.WriteString(event.Detail)
	}
	if len(event.Fields) > 0 {
		keys := make([]string, 0, len(event.Fields))
		for key := range event.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			builder.WriteString(" ")
			builder.WriteString(key)
			builder.WriteString("=")
			builder.WriteString(event.Fields[key])
		}
	}
	return builder.String()
}

// CopyHumanLines reads path and writes one FormatLine per event to writer.
func CopyHumanLines(writer io.Writer, path string) error {
	events, err := Read(path)
	if err != nil {
		return err
	}
	for _, event := range events {
		if _, err := fmt.Fprintln(writer, FormatLine(event)); err != nil {
			return err
		}
	}
	return nil
}
