package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

// injectCredentials looks up the model's provider in the credentials store
// and copies the API key plus any persisted ExtraEnv into config.Env so the
// child process (typically agentd) can read them via os.Getenv.
//
// The credential env name is taken from config.Model.APIKeyEnv if set,
// otherwise from the model catalog's default for the provider. Base URL
// overrides from the credential record win over the Agentfile default so an
// operator can `agentctl model openai auth login --endpoint <gateway>` and
// have every agent re-routed.
func (a *App) injectCredentials(config *agent.Config) {
	provider := strings.ToLower(strings.TrimSpace(config.Model.Provider))
	if provider == "" {
		return
	}
	stored, ok, err := a.credentials.Get(provider)
	if err != nil || !ok {
		return
	}

	if config.Env == nil {
		config.Env = map[string]string{}
	}

	if stored.Endpoint != "" {
		config.Model.BaseURL = stored.Endpoint
	}

	envName := strings.TrimSpace(config.Model.APIKeyEnv)
	if envName == "" {
		envName = a.models.Default(provider).CredentialEnv
	}
	if stored.APIKey != "" && envName != "" {
		config.Env[envName] = stored.APIKey
		if config.Model.APIKeyEnv == "" {
			config.Model.APIKeyEnv = envName
		}
	}
	for key, value := range stored.ExtraEnv {
		config.Env[key] = value
	}
}

// defaultExec injects ["agentd","--config",<configPath>,"--addr",<addr>] when
// the Agentfile didn't supply EXEC. The address is derived from the agent's
// `ENDPOINT http <url>` directive, falling back to 127.0.0.1:8088.
func defaultExec(config *agent.Config, configPath string) {
	if len(config.Exec) > 0 && strings.TrimSpace(config.Exec[0]) != "" {
		return
	}
	addr := endpointAddress(*config)
	config.Exec = []string{"agentd", "--config", configPath, "--addr", addr}
}

func endpointAddress(config agent.Config) string {
	for _, endpoint := range config.Endpoints {
		if strings.EqualFold(endpoint.Name, "http") {
			if value := agent.EndpointHostPort(endpoint); value != "" {
				return value
			}
		}
	}
	return "127.0.0.1:8088"
}

// writeConfigFile dumps config as indented JSON at path with mode 0600 so
// agentd can read it back without re-parsing the Agentfile. The directory is
// created if missing.
func writeConfigFile(path string, config agent.Config) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
