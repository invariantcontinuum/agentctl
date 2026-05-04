package model

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type Provider struct {
	Ref           string
	Kind          string
	Runtime       string
	Endpoint      string
	Auth          string
	CredentialEnv string
	Description   string
}

type Catalog struct {
	providers []Provider
}

func DefaultCatalog() Catalog {
	return Catalog{providers: []Provider{
		{
			Ref:           "openai:default",
			Kind:          "hosted",
			Runtime:       "openai",
			Endpoint:      "https://api.openai.com/v1",
			Auth:          "api_key",
			CredentialEnv: "OPENAI_API_KEY",
			Description:   "OpenAI-compatible hosted model provider.",
		},
		{
			Ref:           "anthropic:default",
			Kind:          "hosted",
			Runtime:       "anthropic",
			Endpoint:      "https://api.anthropic.com",
			Auth:          "api_key",
			CredentialEnv: "ANTHROPIC_API_KEY",
			Description:   "Anthropic hosted model provider.",
		},
		{
			Ref:           "gemini:default",
			Kind:          "hosted",
			Runtime:       "gemini",
			Endpoint:      "https://generativelanguage.googleapis.com",
			Auth:          "api_key_or_oauth",
			CredentialEnv: "GEMINI_API_KEY",
			Description:   "Google Gemini hosted model provider; OAuth can be layered by driver config.",
		},
		{
			Ref:         "vllm:local",
			Kind:        "local",
			Runtime:     "vllm",
			Endpoint:    "http://localhost:8000/v1",
			Auth:        "none",
			Description: "Local OpenAI-compatible vLLM endpoint.",
		},
		{
			Ref:         "llamacpp:local",
			Kind:        "local",
			Runtime:     "llamacpp",
			Endpoint:    "http://localhost:8102/v1",
			Auth:        "none",
			Description: "Local llama.cpp OpenAI-compatible endpoint.",
		},
	}}
}

func (c Catalog) List() []Provider {
	providers := append([]Provider{}, c.providers...)
	sort.Slice(providers, func(left, right int) bool {
		return providers[left].Ref < providers[right].Ref
	})
	return providers
}

// Default returns the canonical provider definition for a short name like
// "anthropic" or "openai". It tolerates both the bare name and an explicit
// "<name>:<variant>" reference. An empty Provider is returned when no entry
// exists, so callers can decide whether the absence is fatal.
func (c Catalog) Default(name string) Provider {
	prefix := strings.ToLower(strings.TrimSpace(name))
	if prefix == "" {
		return Provider{}
	}
	if hit, ok := c.findExact(prefix); ok {
		return hit
	}
	for _, candidate := range c.providers {
		ref := strings.ToLower(candidate.Ref)
		if ref == prefix+":default" || ref == prefix+":local" || ref == prefix {
			return candidate
		}
		if strings.HasPrefix(ref, prefix+":") {
			return candidate
		}
	}
	return Provider{}
}

func (c Catalog) findExact(ref string) (Provider, bool) {
	for _, candidate := range c.providers {
		if strings.EqualFold(candidate.Ref, ref) {
			return candidate, true
		}
	}
	return Provider{}, false
}

func (c Catalog) WriteTable(writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "%-20s %-8s %-12s %-16s %-18s %s\n", "REF", "KIND", "RUNTIME", "AUTH", "CREDENTIAL", "ENDPOINT"); err != nil {
		return err
	}
	for _, provider := range c.List() {
		credential := provider.CredentialEnv
		if credential == "" {
			credential = "-"
		}
		if _, err := fmt.Fprintf(writer, "%-20s %-8s %-12s %-16s %-18s %s\n", provider.Ref, provider.Kind, provider.Runtime, provider.Auth, credential, provider.Endpoint); err != nil {
			return err
		}
	}
	return nil
}
