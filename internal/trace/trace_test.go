package trace

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileWriterWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.trace")
	writer := NewFileWriter(path)

	first := Event{
		Time:   time.Unix(100, 0).UTC(),
		Kind:   KindRun,
		Agent:  "planner-1",
		Detail: "pid=42",
		Fields: map[string]string{"workdir": "."},
	}
	if err := writer.Write(first); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	second := Event{Kind: KindStop, Agent: "planner-1", Detail: "pid=42"}
	if err := writer.Write(second); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	events, err := Read(path)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Kind != KindRun || events[0].Agent != "planner-1" {
		t.Fatalf("event[0] = %+v, want run/planner-1", events[0])
	}
	if events[0].Fields["workdir"] != "." {
		t.Fatalf("event[0].Fields[workdir] = %q, want '.'", events[0].Fields["workdir"])
	}
	if events[1].Time.IsZero() {
		t.Fatal("event[1].Time is zero, expected default-injected timestamp")
	}
}

func TestReadMissingFileReturnsEmpty(t *testing.T) {
	events, err := Read(filepath.Join(t.TempDir(), "missing.trace"))
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}

func TestFormatLineDeterministicFieldOrder(t *testing.T) {
	event := Event{
		Time:   time.Unix(100, 0).UTC(),
		Kind:   KindTool,
		Agent:  "coder-1",
		Detail: "search",
		Fields: map[string]string{"latency_ms": "12", "status": "ok"},
	}
	got := FormatLine(event)
	want := "1970-01-01T00:01:40Z tool agent=coder-1 search latency_ms=12 status=ok"
	if got != want {
		t.Fatalf("FormatLine = %q, want %q", got, want)
	}
}

func TestCopyHumanLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.trace")
	writer := NewFileWriter(path)
	if err := writer.Write(Event{Time: time.Unix(100, 0).UTC(), Kind: KindRun, Agent: "coder-1", Detail: "pid=42"}); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := writer.Write(Event{Time: time.Unix(101, 0).UTC(), Kind: KindStop, Agent: "coder-1", Detail: "pid=42"}); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	var buffer bytes.Buffer
	if err := CopyHumanLines(&buffer, path); err != nil {
		t.Fatalf("CopyHumanLines returned error: %v", err)
	}
	output := buffer.String()
	if !strings.Contains(output, "run agent=coder-1") {
		t.Fatalf("output missing run line: %s", output)
	}
	if !strings.Contains(output, "stop agent=coder-1") {
		t.Fatalf("output missing stop line: %s", output)
	}
}

func TestEmptyWriterPathIsNoOp(t *testing.T) {
	writer := NewFileWriter("")
	if err := writer.Write(Event{Kind: KindRun}); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
}
