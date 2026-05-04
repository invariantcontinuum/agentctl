// Package credentials persists per-provider API keys, endpoints, and
// provider-specific extra environment variables.
//
// The Anthropic Agent SDK and OpenAI Agents SDK both authenticate by API key
// (e.g. ANTHROPIC_API_KEY, OPENAI_API_KEY). Anthropic also supports Amazon
// Bedrock (CLAUDE_CODE_USE_BEDROCK), Google Vertex (CLAUDE_CODE_USE_VERTEX),
// and Microsoft Foundry (CLAUDE_CODE_USE_FOUNDRY). The Store stays neutral so
// any of those switches can be persisted as ExtraEnv entries.
//
// The on-disk file lives at ${XDG_CONFIG_HOME}/agentctl/credentials.json with
// 0600 permissions. The file format is intentionally JSON so an operator can
// inspect or edit it with a text editor when needed.
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProviderCredentials is one row in the store.
type ProviderCredentials struct {
	APIKey   string            `json:"api_key,omitempty"`
	Endpoint string            `json:"endpoint,omitempty"`
	ExtraEnv map[string]string `json:"extra_env,omitempty"`
}

// Store is the minimal surface the CLI needs. The interface keeps the file
// implementation testable and lets us layer keychain or Vault backends later.
type Store interface {
	Get(provider string) (ProviderCredentials, bool, error)
	Set(provider string, credentials ProviderCredentials) error
	Delete(provider string) error
	List() ([]string, error)
}

// JSONStore is the default file-backed Store.
type JSONStore struct {
	path string
}

type fileState struct {
	Providers map[string]ProviderCredentials `json:"providers"`
}

// ErrNotFound is returned by Get and Delete when the provider has no
// credentials recorded.
var ErrNotFound = errors.New("provider credentials not found")

// NewJSONStore returns a JSONStore bound to path. The file is created lazily
// on the first Set call.
func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

// Path exposes the file location for callers that want to print it.
func (s *JSONStore) Path() string { return s.path }

// DefaultPath returns the conventional credentials file location next to the
// agentctl state file.
func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "agentctl", "credentials.json"), nil
}

// Get returns the credentials for provider, plus a boolean indicating
// presence. The boolean keeps "no record" distinct from "io error".
func (s *JSONStore) Get(provider string) (ProviderCredentials, bool, error) {
	provider = normalize(provider)
	state, err := s.load()
	if err != nil {
		return ProviderCredentials{}, false, err
	}
	credentials, ok := state.Providers[provider]
	if !ok {
		return ProviderCredentials{}, false, nil
	}
	return credentials, true, nil
}

// Set replaces the credentials for provider. An empty APIKey is allowed so
// local-only providers (vllm, llamacpp) can persist just an endpoint.
func (s *JSONStore) Set(provider string, credentials ProviderCredentials) error {
	provider = normalize(provider)
	if provider == "" {
		return errors.New("provider name is required")
	}
	state, err := s.load()
	if err != nil {
		return err
	}
	if state.Providers == nil {
		state.Providers = map[string]ProviderCredentials{}
	}
	state.Providers[provider] = credentials
	return s.save(state)
}

// Delete removes the credentials for provider.
func (s *JSONStore) Delete(provider string) error {
	provider = normalize(provider)
	state, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := state.Providers[provider]; !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, provider)
	}
	delete(state.Providers, provider)
	return s.save(state)
}

// List returns every provider name in deterministic alphabetical order.
func (s *JSONStore) List() ([]string, error) {
	state, err := s.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(state.Providers))
	for name := range state.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *JSONStore) load() (fileState, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{Providers: map[string]ProviderCredentials{}}, nil
	}
	if err != nil {
		return fileState{}, err
	}
	if len(data) == 0 {
		return fileState{Providers: map[string]ProviderCredentials{}}, nil
	}
	var state fileState
	if err := json.Unmarshal(data, &state); err != nil {
		return fileState{}, err
	}
	if state.Providers == nil {
		state.Providers = map[string]ProviderCredentials{}
	}
	return state, nil
}

func (s *JSONStore) save(state fileState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tempPath := s.path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, s.path)
}

func normalize(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}
