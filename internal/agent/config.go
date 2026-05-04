package agent

import (
	"os"
	"sort"
)

type Config struct {
	Image        string            `json:"image,omitempty"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Model        Model             `json:"model,omitempty"`
	Skills       []Skill           `json:"skills,omitempty"`
	MCPServers   []MCPServer       `json:"mcp_servers,omitempty"`
	VectorStores []RAGSource       `json:"vector_stores,omitempty"`
	GraphStores  []RAGSource       `json:"graph_stores,omitempty"`
	Memories     []Memory          `json:"memories,omitempty"`
	Loop         Loop              `json:"loop"`
	Endpoints    []Endpoint        `json:"endpoints,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Exec         []string          `json:"exec"`
}

type Skill struct {
	Source string `json:"source"`
}

type Model struct {
	Provider      string `json:"provider,omitempty"`
	Name          string `json:"name,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	Auth          string `json:"auth,omitempty"`
	CredentialEnv string `json:"credential_env,omitempty"`
}

type MCPServer struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type RAGSource struct {
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	DSN        string `json:"dsn"`
	Collection string `json:"collection,omitempty"`
}

type Memory struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
}

type Loop struct {
	Strategy string `json:"strategy"`
	MaxSteps int    `json:"max_steps"`
}

type Endpoint struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (c Config) Command() (string, []string) {
	if len(c.Exec) == 0 {
		return "", nil
	}
	return c.Exec[0], c.Exec[1:]
}

func (c Config) EnvList(base []string) []string {
	if len(base) == 0 {
		base = os.Environ()
	}
	if len(c.Env) == 0 {
		return base
	}

	keys := make([]string, 0, len(c.Env))
	for key := range c.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := append([]string{}, base...)
	for _, key := range keys {
		env = append(env, key+"="+c.Env[key])
	}
	return env
}
