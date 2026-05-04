package agentsdk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// SequentialAgent runs each Child in declaration order, threading the
// Final text from one child as the Input to the next. Mirrors ADK-Go's
// SequentialAgent and lets a planner-then-executor pipeline read top-down.
//
// All children share the same Session so later children see earlier
// transcripts. To isolate a child, wrap it in IsolatedAgent.
type SequentialAgent struct {
	AgentName string
	Children  []Runnable
}

// Name implements Runnable.
func (a *SequentialAgent) Name() string {
	if a.AgentName == "" {
		return "sequential"
	}
	return a.AgentName
}

// Run implements Runnable.
func (a *SequentialAgent) Run(ctx context.Context, session Session, input string) (RunResult, error) {
	if len(a.Children) == 0 {
		return RunResult{}, errors.New("sequential: no children")
	}
	combined := RunResult{AgentName: a.Name()}
	current := input
	for index, child := range a.Children {
		select {
		case <-ctx.Done():
			return combined, ctx.Err()
		default:
		}
		result, err := child.Run(ctx, session, current)
		if err != nil {
			return combined, fmt.Errorf("sequential[%d] %s: %w", index, child.Name(), err)
		}
		combined.Children = append(combined.Children, result)
		combined.Steps += result.Steps
		combined.Final = result.Final
		combined.StopReason = result.StopReason
		if result.Final != "" {
			current = result.Final
		}
	}
	return combined, nil
}

// ParallelAgent runs every child against the same input concurrently and
// returns each child's result. The Final text is the join of all children's
// finals so a downstream consumer can still read a single string. Each child
// gets its own ephemeral session so concurrent appends don't race.
type ParallelAgent struct {
	AgentName string
	Children  []Runnable
	Separator string
}

// Name implements Runnable.
func (a *ParallelAgent) Name() string {
	if a.AgentName == "" {
		return "parallel"
	}
	return a.AgentName
}

// Run implements Runnable.
func (a *ParallelAgent) Run(ctx context.Context, _ Session, input string) (RunResult, error) {
	if len(a.Children) == 0 {
		return RunResult{}, errors.New("parallel: no children")
	}

	separator := a.Separator
	if separator == "" {
		separator = "\n\n"
	}

	type slot struct {
		index  int
		result RunResult
		err    error
	}
	results := make(chan slot, len(a.Children))
	var wait sync.WaitGroup

	for index, child := range a.Children {
		wait.Add(1)
		go func(index int, child Runnable) {
			defer wait.Done()
			session := NewMemorySession(fmt.Sprintf("%s-%d", a.Name(), index))
			result, err := child.Run(ctx, session, input)
			results <- slot{index: index, result: result, err: err}
		}(index, child)
	}
	wait.Wait()
	close(results)

	ordered := make([]RunResult, len(a.Children))
	finals := make([]string, len(a.Children))
	combined := RunResult{AgentName: a.Name()}
	childErrors := make([]error, 0)
	for entry := range results {
		if entry.err != nil {
			childErrors = append(childErrors, fmt.Errorf("parallel[%d] %s: %w", entry.index, a.Children[entry.index].Name(), entry.err))
			continue
		}
		ordered[entry.index] = entry.result
		finals[entry.index] = entry.result.Final
		combined.Steps += entry.result.Steps
	}
	combined.Children = ordered
	combined.Final = strings.Join(filterEmpty(finals), separator)
	combined.StopReason = StopReasonEndTurn
	if len(childErrors) > 0 {
		// Successful children's results stay populated so callers can
		// inspect partial progress alongside the joined error.
		return combined, errors.Join(childErrors...)
	}
	return combined, nil
}

// LoopAgent repeatedly runs Child until either Predicate returns true or
// MaxIterations is reached. Mirrors ADK-Go's LoopAgent. The Child's Final
// is fed back as the next iteration's Input so a self-correcting loop just
// works.
type LoopAgent struct {
	AgentName     string
	Child         Runnable
	Predicate     func(RunResult) bool
	MaxIterations int
}

// Name implements Runnable.
func (a *LoopAgent) Name() string {
	if a.AgentName == "" {
		return "loop"
	}
	return a.AgentName
}

// Run implements Runnable.
func (a *LoopAgent) Run(ctx context.Context, session Session, input string) (RunResult, error) {
	if a.Child == nil {
		return RunResult{}, errors.New("loop: child is nil")
	}
	max := a.MaxIterations
	if max <= 0 {
		max = 4
	}
	combined := RunResult{AgentName: a.Name()}
	current := input
	for iteration := 0; iteration < max; iteration++ {
		select {
		case <-ctx.Done():
			return combined, ctx.Err()
		default:
		}
		result, err := a.Child.Run(ctx, session, current)
		if err != nil {
			return combined, fmt.Errorf("loop iteration %d: %w", iteration, err)
		}
		combined.Children = append(combined.Children, result)
		combined.Steps += result.Steps
		combined.Final = result.Final
		combined.StopReason = result.StopReason
		if a.Predicate != nil && a.Predicate(result) {
			return combined, nil
		}
		if result.Final != "" {
			current = result.Final
		}
	}
	combined.StopReason = "max_iterations"
	return combined, nil
}

// HandoffAgent is a one-shot router: Router returns a final text whose
// content selects exactly one Child, and that Child receives the original
// Input. Mirrors the OpenAI Agents SDK "handoffs" pattern. Selection is
// case-insensitive substring match against the Children map keys.
type HandoffAgent struct {
	AgentName string
	Router    Runnable
	Children  map[string]Runnable
	Default   string
}

// Name implements Runnable.
func (a *HandoffAgent) Name() string {
	if a.AgentName == "" {
		return "handoff"
	}
	return a.AgentName
}

// Run implements Runnable.
func (a *HandoffAgent) Run(ctx context.Context, session Session, input string) (RunResult, error) {
	if a.Router == nil {
		return RunResult{}, errors.New("handoff: router is nil")
	}
	if len(a.Children) == 0 {
		return RunResult{}, errors.New("handoff: no children")
	}
	routerSession := NewMemorySession(a.Name() + "-router")
	routerResult, err := a.Router.Run(ctx, routerSession, input)
	if err != nil {
		return RunResult{}, fmt.Errorf("handoff router: %w", err)
	}
	choice := pickHandoffChild(routerResult.Final, a.Children, a.Default)
	if choice == "" {
		return RunResult{}, fmt.Errorf("handoff: router returned no usable choice (%q)", routerResult.Final)
	}
	child := a.Children[choice]
	childResult, err := child.Run(ctx, session, input)
	if err != nil {
		return RunResult{}, fmt.Errorf("handoff target %s: %w", choice, err)
	}
	return RunResult{
		AgentName:  a.Name(),
		Final:      childResult.Final,
		Steps:      routerResult.Steps + childResult.Steps,
		StopReason: childResult.StopReason,
		Children:   []RunResult{routerResult, childResult},
	}, nil
}

// IsolatedAgent wraps a Runnable so each Run gets its own fresh
// MemorySession instead of sharing the parent's. Use it inside a
// SequentialAgent when one stage shouldn't see another's transcript.
type IsolatedAgent struct {
	Inner Runnable
}

// Name implements Runnable.
func (a *IsolatedAgent) Name() string {
	if a.Inner == nil {
		return "isolated"
	}
	return a.Inner.Name()
}

// Run implements Runnable.
func (a *IsolatedAgent) Run(ctx context.Context, _ Session, input string) (RunResult, error) {
	if a.Inner == nil {
		return RunResult{}, errors.New("isolated: inner is nil")
	}
	return a.Inner.Run(ctx, NewMemorySession(a.Name()), input)
}

func pickHandoffChild(routerFinal string, children map[string]Runnable, fallback string) string {
	candidate := strings.ToLower(strings.TrimSpace(routerFinal))
	if candidate == "" {
		return fallback
	}
	// Direct equality wins.
	for key := range children {
		if strings.EqualFold(key, candidate) {
			return key
		}
	}
	// Then substring match against the lowercased candidate.
	for key := range children {
		if strings.Contains(candidate, strings.ToLower(key)) {
			return key
		}
	}
	return fallback
}

func filterEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
