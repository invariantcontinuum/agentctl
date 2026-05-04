package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSetGetDeleteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(filepath.Join(dir, "credentials.json"))

	creds := ProviderCredentials{
		APIKey:   "sk-test-1",
		Endpoint: "https://api.anthropic.com",
		ExtraEnv: map[string]string{"CLAUDE_CODE_USE_BEDROCK": "1"},
	}
	if err := store.Set("ANTHROPIC", creds); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, ok, err := store.Get("anthropic")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("Get reported missing after Set")
	}
	if got.APIKey != creds.APIKey || got.Endpoint != creds.Endpoint {
		t.Fatalf("Get = %+v, want %+v", got, creds)
	}
	if got.ExtraEnv["CLAUDE_CODE_USE_BEDROCK"] != "1" {
		t.Fatalf("ExtraEnv missing key: %+v", got.ExtraEnv)
	}

	if err := store.Delete("Anthropic"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, ok, err := store.Get("anthropic"); err != nil || ok {
		t.Fatalf("Get after Delete = ok=%t err=%v", ok, err)
	}
}

func TestDeleteUnknownReturnsErrNotFound(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "credentials.json"))
	err := store.Delete("missing")
	if err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete err = %v, want ErrNotFound", err)
	}
}

func TestListReturnsSortedNames(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "credentials.json"))
	for _, name := range []string{"openai", "anthropic", "vllm"} {
		if err := store.Set(name, ProviderCredentials{Endpoint: "x"}); err != nil {
			t.Fatalf("Set %s returned error: %v", name, err)
		}
	}
	names, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	want := []string{"anthropic", "openai", "vllm"}
	for index, name := range want {
		if names[index] != name {
			t.Fatalf("List = %v, want %v", names, want)
		}
	}
}

func TestSetWritesWithMode0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	store := NewJSONStore(path)
	if err := store.Set("anthropic", ProviderCredentials{APIKey: "x"}); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestGetMissingReturnsFalseNoError(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "credentials.json"))
	creds, ok, err := store.Get("nope")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatalf("Get reported ok=true on empty store: %+v", creds)
	}
}
