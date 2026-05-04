package agentsdk

import (
	"context"
	"regexp"
	"testing"
)

func TestRegexGuardBlocksMatch(t *testing.T) {
	guard := &RegexGuard{
		GuardName: "secret",
		Pattern:   regexp.MustCompile(`(?i)password`),
		Reason:    "redact secrets",
	}
	if err := guard.Check(context.Background(), "your PASSWORD is hunter2"); err == nil {
		t.Fatalf("expected guard to fire")
	}
	if err := guard.Check(context.Background(), "all good"); err != nil {
		t.Fatalf("benign content blocked: %v", err)
	}
}

func TestMaxLengthGuard(t *testing.T) {
	guard := &MaxLengthGuard{GuardName: "len", Max: 4}
	if err := guard.Check(context.Background(), "ok"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := guard.Check(context.Background(), "way too long"); err == nil {
		t.Fatalf("expected guard to fire")
	}
}
