package agentsdk

import (
	"path/filepath"
	"testing"
)

func TestMemorySessionAppendAndCopy(t *testing.T) {
	session := NewMemorySession("mem")
	if err := session.Append(UserMessage("first")); err != nil {
		t.Fatalf("append: %v", err)
	}
	got := session.Messages()
	got[0].Content = nil // mutate copy
	if session.Messages()[0].Content == nil {
		t.Fatalf("Messages did not return defensive copy")
	}
}

func TestMemorySessionReset(t *testing.T) {
	session := NewMemorySession("mem")
	_ = session.Append(UserMessage("a"))
	_ = session.Reset()
	if len(session.Messages()) != 0 {
		t.Fatalf("reset did not clear")
	}
}

func TestFileSessionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	session, err := NewFileSession("agent-x", path)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := session.Append(UserMessage("hello")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := session.Append(AssistantMessage(TextBlock("hi back"))); err != nil {
		t.Fatalf("append assistant: %v", err)
	}

	reopened, err := NewFileSession("agent-x", path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	messages := reopened.Messages()
	if len(messages) != 2 {
		t.Fatalf("len = %d, want 2", len(messages))
	}
	if messages[0].FirstText() != "hello" {
		t.Fatalf("user text = %q", messages[0].FirstText())
	}
	if messages[1].Role != RoleAssistant || messages[1].FirstText() != "hi back" {
		t.Fatalf("assistant lost: %+v", messages[1])
	}
}

func TestFileSessionResetTruncates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	session, err := NewFileSession("agent-y", path)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_ = session.Append(UserMessage("a"))
	if err := session.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if len(session.Messages()) != 0 {
		t.Fatalf("reset did not clear in-memory")
	}
	reopened, err := NewFileSession("agent-y", path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if len(reopened.Messages()) != 0 {
		t.Fatalf("reset did not clear on disk")
	}
}
