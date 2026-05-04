package agent

import (
	"fmt"
	"net/url"
	"strings"
)

type Validator interface {
	Validate(Config) error
}

type ConfigValidator struct{}

type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "invalid agent config: " + strings.Join(e.Problems, "; ")
}

func (ConfigValidator) Validate(config Config) error {
	var problems []string

	if strings.TrimSpace(config.Name) == "" {
		problems = append(problems, "AGENT is required")
	}
	if strings.TrimSpace(config.Type) == "" {
		problems = append(problems, "TYPE is required")
	}
	if len(config.Exec) == 0 || strings.TrimSpace(config.Exec[0]) == "" {
		problems = append(problems, "EXEC is required")
	}
	if strings.TrimSpace(config.Loop.Strategy) == "" {
		problems = append(problems, "LOOP strategy is required")
	}
	if config.Loop.MaxSteps <= 0 {
		problems = append(problems, "LOOP max_steps must be positive")
	}
	if strings.TrimSpace(config.Model.Provider) != "" && strings.TrimSpace(config.Model.Name) == "" {
		problems = append(problems, "MODEL name is required when provider is set")
	}
	if strings.TrimSpace(config.Model.Endpoint) != "" && !validURL(config.Model.Endpoint) {
		problems = append(problems, "MODEL endpoint is invalid")
	}

	for _, server := range config.MCPServers {
		if server.Name == "" {
			problems = append(problems, "MCP server name is required")
		}
		switch server.Transport {
		case MCPTransportHTTP:
			if !validURL(server.URL) {
				problems = append(problems, fmt.Sprintf("MCP %q URL is invalid", server.Name))
			}
		case MCPTransportStdio:
			if strings.TrimSpace(server.Command) == "" {
				problems = append(problems, fmt.Sprintf("MCP %q stdio command is required", server.Name))
			}
		case "":
			problems = append(problems, fmt.Sprintf("MCP %q transport is required (http or stdio)", server.Name))
		default:
			problems = append(problems, fmt.Sprintf("MCP %q unknown transport %q", server.Name, server.Transport))
		}
	}
	for _, endpoint := range config.Endpoints {
		if endpoint.Name == "" {
			problems = append(problems, "ENDPOINT name is required")
		}
		if !validURL(endpoint.URL) {
			problems = append(problems, fmt.Sprintf("ENDPOINT %q URL is invalid", endpoint.Name))
		}
	}

	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func validURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}
