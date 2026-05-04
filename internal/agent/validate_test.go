package agent

import "testing"

func TestConfigValidatorRejectsMissingLoopMaxSteps(t *testing.T) {
	config := Config{
		Name: "planner",
		Type: "planner",
		Loop: Loop{Strategy: "react"},
		Exec: []string{"sleep", "60"},
	}

	err := (ConfigValidator{}).Validate(config)
	if err == nil {
		t.Fatal("Validate returned nil error")
	}
}

func TestConfigValidatorAcceptsMinimalConfig(t *testing.T) {
	config := Config{
		Name: "planner",
		Type: "planner",
		Loop: Loop{Strategy: "react", MaxSteps: 10},
		Exec: []string{"sleep", "60"},
	}

	if err := (ConfigValidator{}).Validate(config); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
