package compose

import (
	"strings"
	"testing"
)

func TestParseValidDocument(t *testing.T) {
	source := `
# planner-led team
COMPOSE planner-team
AGENT planner FILE=./planner/Agentfile
AGENT coder FILE=./coder/Agentfile DEPENDS_ON=planner
AGENT reviewer FILE=./reviewer/Agentfile DEPENDS_ON=coder
`
	document, err := NewParser().Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if document.Name != "planner-team" {
		t.Fatalf("Name = %q, want planner-team", document.Name)
	}
	if len(document.Services) != 3 {
		t.Fatalf("len(Services) = %d, want 3", len(document.Services))
	}
	if document.Services[1].DependsOn[0] != "planner" {
		t.Fatalf("Services[1].DependsOn = %v, want [planner]", document.Services[1].DependsOn)
	}
}

func TestParseRejectsUnknownDirective(t *testing.T) {
	_, err := NewParser().Parse(strings.NewReader("COMPOSE team\nFOO bar"))
	if err == nil || !strings.Contains(err.Error(), "unknown directive") {
		t.Fatalf("err = %v, want unknown directive failure", err)
	}
}

func TestValidateRequiresName(t *testing.T) {
	document := Document{Services: []Service{{Name: "a", File: "x"}}}
	if err := document.Validate(); err == nil {
		t.Fatal("Validate returned nil, want error")
	}
}

func TestValidateRejectsDuplicateService(t *testing.T) {
	document := Document{Name: "team", Services: []Service{{Name: "a", File: "x"}, {Name: "a", File: "y"}}}
	if err := document.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Validate err = %v, want duplicate", err)
	}
}

func TestValidateRejectsMissingDependency(t *testing.T) {
	document := Document{Name: "team", Services: []Service{{Name: "a", File: "x", DependsOn: []string{"missing"}}}}
	if err := document.Validate(); err == nil || !strings.Contains(err.Error(), "unknown service") {
		t.Fatalf("Validate err = %v, want unknown service", err)
	}
}

func TestPlanReturnsTopologicalOrder(t *testing.T) {
	document := Document{
		Name: "team",
		Services: []Service{
			{Name: "reviewer", File: "r", DependsOn: []string{"coder"}},
			{Name: "planner", File: "p"},
			{Name: "coder", File: "c", DependsOn: []string{"planner"}},
		},
	}
	plan, err := document.Plan()
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	got := []string{plan[0].Name, plan[1].Name, plan[2].Name}
	want := []string{"planner", "coder", "reviewer"}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("Plan = %v, want %v", got, want)
		}
	}
}

func TestPlanDetectsCycle(t *testing.T) {
	document := Document{
		Name: "team",
		Services: []Service{
			{Name: "a", File: "a", DependsOn: []string{"b"}},
			{Name: "b", File: "b", DependsOn: []string{"a"}},
		},
	}
	if _, err := document.Plan(); err != ErrCyclicDependency {
		t.Fatalf("Plan err = %v, want ErrCyclicDependency", err)
	}
}
