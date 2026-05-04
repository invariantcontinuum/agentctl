package agentsdk

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunnable is the minimum implementation of Runnable for orchestrator
// tests. It records every input it received and returns a configured Final.
type fakeRunnable struct {
	name      string
	finalText string
	err       error
	inputs    []string
}

func (f *fakeRunnable) Name() string { return f.name }

func (f *fakeRunnable) Run(_ context.Context, _ Session, input string) (RunResult, error) {
	f.inputs = append(f.inputs, input)
	if f.err != nil {
		return RunResult{}, f.err
	}
	return RunResult{AgentName: f.name, Final: f.finalText, Steps: 1, StopReason: StopReasonEndTurn}, nil
}

func TestSequentialAgentThreadsFinalAsNextInput(t *testing.T) {
	first := &fakeRunnable{name: "first", finalText: "from-first"}
	second := &fakeRunnable{name: "second", finalText: "from-second"}
	agent := &SequentialAgent{AgentName: "pipe", Children: []Runnable{first, second}}

	result, err := agent.Run(context.Background(), NewMemorySession("seq"), "input")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if first.inputs[0] != "input" {
		t.Fatalf("first input = %q", first.inputs[0])
	}
	if second.inputs[0] != "from-first" {
		t.Fatalf("second input = %q", second.inputs[0])
	}
	if result.Final != "from-second" {
		t.Fatalf("final = %q", result.Final)
	}
	if len(result.Children) != 2 {
		t.Fatalf("children = %d", len(result.Children))
	}
}

func TestSequentialAgentSurfacesChildError(t *testing.T) {
	failing := &fakeRunnable{name: "fail", err: errors.New("nope")}
	agent := &SequentialAgent{AgentName: "pipe", Children: []Runnable{failing}}
	if _, err := agent.Run(context.Background(), NewMemorySession("seq"), "x"); err == nil {
		t.Fatalf("expected child error to bubble")
	}
}

func TestParallelAgentCombinesFinals(t *testing.T) {
	left := &fakeRunnable{name: "left", finalText: "L"}
	right := &fakeRunnable{name: "right", finalText: "R"}
	agent := &ParallelAgent{AgentName: "fan", Children: []Runnable{left, right}, Separator: "|"}

	result, err := agent.Run(context.Background(), nil, "go")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Final != "L|R" {
		t.Fatalf("final = %q", result.Final)
	}
	if len(left.inputs) != 1 || left.inputs[0] != "go" {
		t.Fatalf("left input = %v", left.inputs)
	}
}

func TestLoopAgentStopsOnPredicate(t *testing.T) {
	count := 0
	child := &fakeRunnable{name: "step", finalText: "tick"}
	loop := &LoopAgent{
		AgentName:     "until",
		Child:         child,
		MaxIterations: 5,
		Predicate: func(result RunResult) bool {
			count++
			return count >= 2
		},
	}
	if _, err := loop.Run(context.Background(), NewMemorySession("loop"), "go"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(child.inputs) != 2 {
		t.Fatalf("expected 2 iterations, got %d", len(child.inputs))
	}
}

func TestLoopAgentRespectsMaxIterations(t *testing.T) {
	child := &fakeRunnable{name: "step", finalText: "tick"}
	loop := &LoopAgent{AgentName: "until", Child: child, MaxIterations: 3, Predicate: func(_ RunResult) bool { return false }}

	result, err := loop.Run(context.Background(), NewMemorySession("loop"), "go")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(child.inputs) != 3 {
		t.Fatalf("iterations = %d, want 3", len(child.inputs))
	}
	if result.StopReason != "max_iterations" {
		t.Fatalf("stop reason = %q", result.StopReason)
	}
}

func TestHandoffAgentRoutesByRouterFinal(t *testing.T) {
	router := &fakeRunnable{name: "router", finalText: "billing"}
	billing := &fakeRunnable{name: "billing", finalText: "paid"}
	support := &fakeRunnable{name: "support", finalText: "fixed"}
	agent := &HandoffAgent{
		AgentName: "switchboard",
		Router:    router,
		Children: map[string]Runnable{
			"billing": billing,
			"support": support,
		},
	}
	result, err := agent.Run(context.Background(), NewMemorySession("ho"), "I have a charge issue")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Final != "paid" {
		t.Fatalf("final = %q", result.Final)
	}
	if len(billing.inputs) != 1 {
		t.Fatalf("expected billing to be invoked once")
	}
	if len(support.inputs) != 0 {
		t.Fatalf("support should not be invoked")
	}
}

func TestHandoffAgentFallsBackOnUnknownRoute(t *testing.T) {
	router := &fakeRunnable{name: "router", finalText: "blarghhh"}
	support := &fakeRunnable{name: "support", finalText: "ok"}
	agent := &HandoffAgent{
		AgentName: "switchboard",
		Router:    router,
		Children:  map[string]Runnable{"support": support},
		Default:   "support",
	}
	result, err := agent.Run(context.Background(), NewMemorySession("ho"), "?")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Final != "ok" {
		t.Fatalf("default fallback failed: final=%q", result.Final)
	}
}

func TestIsolatedAgentGivesFreshSession(t *testing.T) {
	child := &fakeRunnable{name: "wrapped", finalText: "ok"}
	isolated := &IsolatedAgent{Inner: child}

	parent := NewMemorySession("parent")
	if err := parent.Append(UserMessage("ancestor")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := isolated.Run(context.Background(), parent, "now"); err != nil {
		t.Fatalf("run: %v", err)
	}
	// Parent session must remain untouched (isolated runs use a fresh
	// memory session under the hood).
	messages := parent.Messages()
	if len(messages) != 1 || messages[0].FirstText() != "ancestor" {
		t.Fatalf("parent session mutated: %+v", messages)
	}
	if !strings.EqualFold(child.inputs[0], "now") {
		t.Fatalf("inner did not receive input: %v", child.inputs)
	}
}
