package cli

import (
	"path/filepath"
	"testing"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/credentials"
	"github.com/invariantcontinuum/agentctl/internal/model"
)

func newTestApp(t *testing.T, store credentials.Store) *App {
	t.Helper()
	app := &App{
		credentials: store,
		models:      model.DefaultCatalog(),
	}
	return app
}

func TestInjectCredentialsAddsAPIKeyEnv(t *testing.T) {
	dir := t.TempDir()
	store := credentials.NewJSONStore(filepath.Join(dir, "creds.json"))
	if err := store.Set("anthropic", credentials.ProviderCredentials{
		APIKey: "sk-test",
	}); err != nil {
		t.Fatalf("set: %v", err)
	}
	app := newTestApp(t, store)

	config := agent.Config{Model: agent.Model{Provider: "anthropic"}}
	app.injectCredentials(&config)

	if config.Env["ANTHROPIC_API_KEY"] != "sk-test" {
		t.Fatalf("expected ANTHROPIC_API_KEY=sk-test, got %v", config.Env)
	}
	if config.Model.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Fatalf("APIKeyEnv = %q", config.Model.APIKeyEnv)
	}
}

func TestInjectCredentialsRespectsExplicitCredentialEnv(t *testing.T) {
	dir := t.TempDir()
	store := credentials.NewJSONStore(filepath.Join(dir, "creds.json"))
	_ = store.Set("openai", credentials.ProviderCredentials{APIKey: "ok"})
	app := newTestApp(t, store)

	config := agent.Config{Model: agent.Model{Provider: "openai", APIKeyEnv: "MY_KEY"}}
	app.injectCredentials(&config)

	if config.Env["MY_KEY"] != "ok" {
		t.Fatalf("expected MY_KEY=ok, got %v", config.Env)
	}
	if _, hasDefault := config.Env["OPENAI_API_KEY"]; hasDefault {
		t.Fatalf("did not expect OPENAI_API_KEY, got %v", config.Env)
	}
}

func TestInjectCredentialsOverridesEndpointAndCopiesExtraEnv(t *testing.T) {
	dir := t.TempDir()
	store := credentials.NewJSONStore(filepath.Join(dir, "creds.json"))
	_ = store.Set("anthropic", credentials.ProviderCredentials{
		APIKey:   "sk",
		Endpoint: "https://gateway.example.com",
		ExtraEnv: map[string]string{"CLAUDE_CODE_USE_BEDROCK": "1"},
	})
	app := newTestApp(t, store)

	config := agent.Config{Model: agent.Model{Provider: "anthropic", BaseURL: "https://api.anthropic.com"}}
	app.injectCredentials(&config)

	if config.Model.BaseURL != "https://gateway.example.com" {
		t.Fatalf("base URL not overridden: %q", config.Model.BaseURL)
	}
	if config.Env["CLAUDE_CODE_USE_BEDROCK"] != "1" {
		t.Fatalf("ExtraEnv missing: %v", config.Env)
	}
}

func TestInjectCredentialsNoOpWithoutProvider(t *testing.T) {
	app := newTestApp(t, credentials.NewJSONStore(filepath.Join(t.TempDir(), "creds.json")))
	config := agent.Config{}
	app.injectCredentials(&config)
	if len(config.Env) != 0 {
		t.Fatalf("env unexpectedly populated: %v", config.Env)
	}
}

func TestDefaultExecInjectsAgentd(t *testing.T) {
	config := agent.Config{
		Endpoints: []agent.Endpoint{{Name: "http", Scheme: "http", Host: "127.0.0.1", Port: 9090}},
	}
	defaultExec(&config, "/tmp/cfg.json")
	wanted := []string{"agentd", "--config", "/tmp/cfg.json", "--addr", "127.0.0.1:9090"}
	if len(config.Exec) != len(wanted) {
		t.Fatalf("exec = %v, want %v", config.Exec, wanted)
	}
	for index, value := range wanted {
		if config.Exec[index] != value {
			t.Fatalf("exec[%d] = %q, want %q", index, config.Exec[index], value)
		}
	}
}

func TestDefaultExecLeavesUserExecIntact(t *testing.T) {
	config := agent.Config{Exec: []string{"/usr/bin/python", "-m", "myagent"}}
	defaultExec(&config, "/tmp/cfg.json")
	if config.Exec[0] != "/usr/bin/python" {
		t.Fatalf("exec rewritten: %v", config.Exec)
	}
}

func TestDefaultExecFallsBackToLoopback(t *testing.T) {
	config := agent.Config{}
	defaultExec(&config, "/tmp/cfg.json")
	if config.Exec[len(config.Exec)-1] != "127.0.0.1:8088" {
		t.Fatalf("addr fallback failed: %v", config.Exec)
	}
}

func TestEndpointAddressStripsScheme(t *testing.T) {
	got := endpointAddress(agent.Config{
		Endpoints: []agent.Endpoint{{Name: "http", Scheme: "https", Host: "example.org", Port: 7000, Path: "/api"}},
	})
	if got != "example.org:7000" {
		t.Fatalf("got %q", got)
	}
}
