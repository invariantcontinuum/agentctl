package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invariantcontinuum/agentctl/internal/credentials"
	"github.com/invariantcontinuum/agentctl/internal/store"
)

func newAuthApp(t *testing.T, dir string) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := New(out, errOut, repo, fakeDriver{})
	app.credentials = credentials.NewJSONStore(filepath.Join(dir, "credentials.json"))
	return app, out, errOut
}

func TestModelAnthropicAuthLoginNonInteractive(t *testing.T) {
	dir := t.TempDir()
	app, out, errOut := newAuthApp(t, dir)

	exitCode := app.Run(context.Background(), []string{"model", "anthropic", "auth", "login", "--api-key", "sk-ant-test", "--no-interactive"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "saved anthropic credentials") {
		t.Fatalf("stdout missing saved confirmation: %s", out.String())
	}

	out.Reset()
	exitCode = app.Run(context.Background(), []string{"model", "anthropic", "auth", "status"})
	if exitCode != 0 {
		t.Fatalf("status exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "logged in") {
		t.Fatalf("status output missing logged in: %s", out.String())
	}
	if strings.Contains(out.String(), "sk-ant-test") {
		t.Fatalf("status output leaked raw key: %s", out.String())
	}
}

func TestModelAuthLoginInteractivePrompt(t *testing.T) {
	dir := t.TempDir()
	app, out, errOut := newAuthApp(t, dir)
	app.stdin = strings.NewReader("sk-ant-from-stdin\n")

	exitCode := app.Run(context.Background(), []string{"model", "anthropic", "auth", "login"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "saved anthropic credentials") {
		t.Fatalf("stdout missing saved line: %s", out.String())
	}

	creds, ok, err := app.credentials.Get("anthropic")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("credentials missing after interactive login")
	}
	if creds.APIKey != "sk-ant-from-stdin" {
		t.Fatalf("APIKey = %q, want sk-ant-from-stdin", creds.APIKey)
	}
}

func TestModelLocalProviderAuthLoginRequiresOnlyEndpoint(t *testing.T) {
	dir := t.TempDir()
	app, out, errOut := newAuthApp(t, dir)

	exitCode := app.Run(context.Background(), []string{"model", "vllm", "auth", "login", "--endpoint", "http://localhost:9000/v1", "--no-interactive"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	creds, ok, err := app.credentials.Get("vllm")
	if err != nil || !ok {
		t.Fatalf("Get ok=%t err=%v", ok, err)
	}
	if creds.Endpoint != "http://localhost:9000/v1" {
		t.Fatalf("Endpoint = %q, want http://localhost:9000/v1", creds.Endpoint)
	}
	if !strings.Contains(out.String(), "saved vllm credentials") {
		t.Fatalf("stdout missing saved line: %s", out.String())
	}
}

func TestModelAuthLogoutRemovesEntry(t *testing.T) {
	dir := t.TempDir()
	app, _, errOut := newAuthApp(t, dir)
	if err := app.credentials.Set("openai", credentials.ProviderCredentials{APIKey: "sk-test", Endpoint: "https://api.openai.com/v1"}); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	exitCode := app.Run(context.Background(), []string{"model", "openai", "auth", "logout"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	if _, ok, _ := app.credentials.Get("openai"); ok {
		t.Fatal("credentials still present after logout")
	}
}

func TestModelLsMarksLoggedInProviders(t *testing.T) {
	dir := t.TempDir()
	app, out, errOut := newAuthApp(t, dir)
	if err := app.credentials.Set("anthropic", credentials.ProviderCredentials{APIKey: "sk-x"}); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	exitCode := app.Run(context.Background(), []string{"model", "ls"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	output := out.String()
	if !strings.Contains(output, "LOGGED IN") {
		t.Fatalf("model ls missing LOGGED IN column: %s", output)
	}
	if !strings.Contains(output, "anthropic:default") {
		t.Fatalf("model ls missing anthropic:default row: %s", output)
	}
}

func TestModelUnknownProviderRejected(t *testing.T) {
	dir := t.TempDir()
	app, _, errOut := newAuthApp(t, dir)

	exitCode := app.Run(context.Background(), []string{"model", "bogus", "auth", "status"})
	if exitCode == 0 {
		t.Fatal("exitCode = 0, want failure for unknown provider")
	}
	if !strings.Contains(errOut.String(), "unknown model provider") {
		t.Fatalf("stderr missing unknown provider message: %s", errOut.String())
	}
}
