// Package logging is agentctl's structured logging adapter.
//
// Two responsibilities:
//
//  1. Emit JSON-Lines log records ({"ts","level","msg","fields"}) for the
//     CLI's own observable actions (start/stop/etc.) so an operator using
//     `--log-level debug` sees deterministic structured output.
//  2. Parse JSON-Lines log records produced by an agent's process so
//     `agentctl logs --level <level>` can filter the file the agent writes.
//
// The agent runtime is expected to write JSON-Lines log records that look
// like the Record type below. Lines that don't parse are surfaced verbatim
// with `level=info` so older / non-conforming log files still display.
package logging

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// Level is the log severity. The zero value is Info so callers can omit it.
type Level int

const (
	LevelDebug Level = iota - 1 // -1
	LevelInfo                   // 0
	LevelWarn                   // 1
	LevelError                  // 2
)

// String returns the canonical lowercase level name used in JSON output.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	}
	return "info"
}

// ParseLevel turns "debug"/"info"/"warn"/"error" (any case) into a Level.
// Empty input returns LevelInfo. Unknown input returns an error so the CLI
// can surface a clear message.
func ParseLevel(value string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return LevelInfo, nil
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error", "err":
		return LevelError, nil
	}
	return LevelInfo, fmt.Errorf("unknown log level %q (want debug|info|warn|error)", value)
}

// Record is one structured log record. Both writer and reader use this shape.
type Record struct {
	Time   time.Time         `json:"ts"`
	Level  string            `json:"level"`
	Msg    string            `json:"msg"`
	Fields map[string]string `json:"fields,omitempty"`
}

// Logger writes Records to w with a minimum level. Records at lower levels
// are dropped. Logger is safe for sequential use; callers that need
// concurrent access should serialize externally.
type Logger struct {
	writer io.Writer
	level  Level
	now    func() time.Time
}

// New returns a Logger writing to w at min level. A nil writer falls back to
// os.Stderr so a misconfigured caller can't silently swallow log output.
func New(w io.Writer, level Level) *Logger {
	if w == nil {
		w = os.Stderr
	}
	return &Logger{writer: w, level: level, now: time.Now}
}

// Level returns the minimum recorded level.
func (l *Logger) Level() Level { return l.level }

// SetLevel adjusts the minimum level after construction.
func (l *Logger) SetLevel(level Level) { l.level = level }

// Debug records msg at debug. Optional kv pairs are flattened into Fields.
func (l *Logger) Debug(msg string, kv ...string) { l.write(LevelDebug, msg, kv) }

// Info records msg at info.
func (l *Logger) Info(msg string, kv ...string) { l.write(LevelInfo, msg, kv) }

// Warn records msg at warn.
func (l *Logger) Warn(msg string, kv ...string) { l.write(LevelWarn, msg, kv) }

// Error records msg at error.
func (l *Logger) Error(msg string, kv ...string) { l.write(LevelError, msg, kv) }

func (l *Logger) write(level Level, msg string, kv []string) {
	if level < l.level {
		return
	}
	record := Record{
		Time:   l.now().UTC(),
		Level:  level.String(),
		Msg:    msg,
		Fields: kvToMap(kv),
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return
	}
	encoded = append(encoded, '\n')
	_, _ = l.writer.Write(encoded)
}

func kvToMap(kv []string) map[string]string {
	if len(kv) == 0 {
		return nil
	}
	fields := make(map[string]string, len(kv)/2)
	for index := 0; index+1 < len(kv); index += 2 {
		fields[kv[index]] = kv[index+1]
	}
	return fields
}

// FilterFile streams the log records in path to writer, dropping anything
// below min. Lines that aren't valid JSON Records are surfaced as info-level
// "raw" lines so non-conforming agent processes still produce visible output.
func FilterFile(writer io.Writer, path string, min Level, asJSON bool) error {
	if path == "" {
		return fmt.Errorf("log path is empty")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return Filter(writer, file, min, asJSON)
}

// Filter is the io.Reader-driven variant of FilterFile.
func Filter(writer io.Writer, reader io.Reader, min Level, asJSON bool) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		record, ok := decodeRecord(raw)
		level, err := ParseLevel(record.Level)
		if !ok || err != nil {
			// Non-JSON or unknown level → treat as info so default filters
			// still print it.
			level = LevelInfo
			record = Record{
				Time:  time.Now().UTC(),
				Level: level.String(),
				Msg:   string(raw),
			}
		}
		if level < min {
			continue
		}
		if asJSON {
			line, err := json.Marshal(record)
			if err != nil {
				return err
			}
			line = append(line, '\n')
			if _, err := writer.Write(line); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintln(writer, formatHumanLine(record)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func decodeRecord(raw []byte) (Record, bool) {
	var record Record
	if err := json.Unmarshal(raw, &record); err != nil {
		return Record{}, false
	}
	if record.Level == "" {
		return Record{}, false
	}
	return record, true
}

func formatHumanLine(record Record) string {
	when := record.Time.UTC().Format(time.RFC3339Nano)
	if record.Time.IsZero() {
		when = "-"
	}
	builder := strings.Builder{}
	builder.WriteString(when)
	builder.WriteString(" ")
	builder.WriteString(strings.ToUpper(record.Level))
	builder.WriteString(" ")
	builder.WriteString(record.Msg)
	if len(record.Fields) > 0 {
		keys := make([]string, 0, len(record.Fields))
		for key := range record.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			builder.WriteString(" ")
			builder.WriteString(key)
			builder.WriteString("=")
			builder.WriteString(record.Fields[key])
		}
	}
	return builder.String()
}
