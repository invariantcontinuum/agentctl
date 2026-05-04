package agentsdk

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

// Guardrail is the OpenAI-Agents-style policy check applied to assistant
// text. Returning a non-nil error aborts the Agent run with the guard's
// message. Implementations are expected to be cheap; long-running checks
// belong in tools, not guards.
type Guardrail interface {
	Name() string
	Check(ctx context.Context, content string) error
}

// RegexGuard rejects content that matches Pattern.
type RegexGuard struct {
	GuardName string
	Pattern   *regexp.Regexp
	Reason    string
}

// Name implements Guardrail.
func (g *RegexGuard) Name() string { return g.GuardName }

// Check implements Guardrail.
func (g *RegexGuard) Check(_ context.Context, content string) error {
	if g.Pattern == nil {
		return nil
	}
	if g.Pattern.MatchString(content) {
		if g.Reason == "" {
			return errors.New("regex match")
		}
		return errors.New(g.Reason)
	}
	return nil
}

// MaxLengthGuard rejects content longer than Max characters.
type MaxLengthGuard struct {
	GuardName string
	Max       int
}

// Name implements Guardrail.
func (g *MaxLengthGuard) Name() string { return g.GuardName }

// Check implements Guardrail.
func (g *MaxLengthGuard) Check(_ context.Context, content string) error {
	if g.Max <= 0 {
		return nil
	}
	if len(content) > g.Max {
		return fmt.Errorf("content length %d exceeds max %d", len(content), g.Max)
	}
	return nil
}
