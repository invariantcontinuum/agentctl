package agentfile

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

// Parser is the interface the CLI consumes. Both Parse(io.Reader) and
// ParseFile(path) are surfaced because FROM inheritance needs the originating
// path to resolve relative parent references.
type Parser interface {
	Parse(io.Reader) (agent.Config, error)
	ParseFile(path string) (agent.Config, error)
}

type LineParser struct{}

func NewParser() LineParser {
	return LineParser{}
}

// errFromUnsupported signals a FROM directive in a stream-only Parse call.
// Stream parses can't resolve a parent path; callers should switch to
// ParseFile.
var errFromUnsupported = errors.New("FROM is only valid via ParseFile (stream parse cannot resolve parent path)")

func (LineParser) Parse(reader io.Reader) (agent.Config, error) {
	return parseInto(reader, "", map[string]struct{}{})
}

// ParseFile reads the Agentfile at path. FROM directives are resolved
// relative to path, recursively, with cycle detection.
func (LineParser) ParseFile(path string) (agent.Config, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return agent.Config{}, err
	}
	return parseFile(absolute, map[string]struct{}{})
}

func parseFile(absolutePath string, visited map[string]struct{}) (agent.Config, error) {
	if _, ok := visited[absolutePath]; ok {
		return agent.Config{}, fmt.Errorf("FROM cycle detected at %s", absolutePath)
	}
	visited[absolutePath] = struct{}{}

	file, err := os.Open(absolutePath)
	if err != nil {
		return agent.Config{}, err
	}
	defer file.Close()
	return parseInto(file, absolutePath, visited)
}

func parseInto(reader io.Reader, absolutePath string, visited map[string]struct{}) (agent.Config, error) {
	config := agent.Config{
		Env:    map[string]string{},
		Labels: map[string]string{},
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if strings.ToUpper(fields[0]) == "FROM" {
			if absolutePath == "" {
				return agent.Config{}, fmt.Errorf("Agentfile:%d: %w", lineNumber, errFromUnsupported)
			}
			if len(fields) != 2 {
				return agent.Config{}, fmt.Errorf("Agentfile:%d: FROM expects exactly one path", lineNumber)
			}
			parentPath := fields[1]
			if !filepath.IsAbs(parentPath) {
				parentPath = filepath.Join(filepath.Dir(absolutePath), parentPath)
			}
			parentAbsolute, err := filepath.Abs(parentPath)
			if err != nil {
				return agent.Config{}, fmt.Errorf("Agentfile:%d: %w", lineNumber, err)
			}
			parent, err := parseFile(parentAbsolute, visited)
			if err != nil {
				return agent.Config{}, fmt.Errorf("Agentfile:%d FROM %s: %w", lineNumber, fields[1], err)
			}
			config = parent
			if config.Env == nil {
				config.Env = map[string]string{}
			}
			if config.Labels == nil {
				config.Labels = map[string]string{}
			}
			continue
		}

		if err := parseLine(&config, line); err != nil {
			return agent.Config{}, fmt.Errorf("Agentfile:%d: %w", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return agent.Config{}, err
	}
	return config, nil
}

func parseLine(config *agent.Config, line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}

	directive := strings.ToUpper(fields[0])
	args := fields[1:]

	switch directive {
	case "IMAGE":
		value, err := singleArg(directive, args)
		if err != nil {
			return err
		}
		config.Image = value
	case "AGENT":
		value, err := singleArg(directive, args)
		if err != nil {
			return err
		}
		config.Name = value
	case "TYPE":
		value, err := singleArg(directive, args)
		if err != nil {
			return err
		}
		config.Type = value
	case "SKILL":
		value, err := singleArg(directive, args)
		if err != nil {
			return err
		}
		config.Skills = append(config.Skills, agent.Skill{Source: value})
	case "MODEL":
		model, err := parseModel(args)
		if err != nil {
			return err
		}
		config.Model = model
	case "MCP":
		server, err := parseMCPLine(args)
		if err != nil {
			return err
		}
		config.MCPServers = append(config.MCPServers, server)
	case "VECTOR":
		if len(args) != 3 && len(args) != 4 {
			return fmt.Errorf("VECTOR expects <name> <provider> <dsn> [collection]")
		}
		source := agent.RAGSource{Name: args[0], Provider: args[1], DSN: args[2]}
		if len(args) == 4 {
			source.Collection = args[3]
		}
		config.VectorStores = append(config.VectorStores, source)
	case "GRAPH":
		if len(args) != 3 {
			return fmt.Errorf("GRAPH expects <name> <provider> <dsn>")
		}
		config.GraphStores = append(config.GraphStores, agent.RAGSource{Name: args[0], Provider: args[1], DSN: args[2]})
	case "MEMORY":
		if len(args) != 3 {
			return fmt.Errorf("MEMORY expects <name> <kind> <source>")
		}
		config.Memories = append(config.Memories, agent.Memory{Name: args[0], Kind: args[1], Source: args[2]})
	case "LOOP":
		if len(args) < 2 {
			return fmt.Errorf("LOOP expects <strategy> max_steps=<positive-int>")
		}
		config.Loop.Strategy = args[0]
		for _, pair := range args[1:] {
			key, value, ok := strings.Cut(pair, "=")
			if !ok {
				return fmt.Errorf("LOOP option %q must use key=value", pair)
			}
			switch key {
			case "max_steps":
				maxSteps, err := strconv.Atoi(value)
				if err != nil {
					return fmt.Errorf("LOOP max_steps must be an integer")
				}
				config.Loop.MaxSteps = maxSteps
			default:
				return fmt.Errorf("unknown LOOP option %q", key)
			}
		}
	case "ENDPOINT":
		if len(args) != 2 {
			return fmt.Errorf("ENDPOINT expects <name> <url>")
		}
		config.Endpoints = append(config.Endpoints, agent.Endpoint{Name: args[0], URL: args[1]})
	case "ENV":
		key, value, err := keyValue(directive, args)
		if err != nil {
			return err
		}
		config.Env[key] = value
	case "LABEL":
		key, value, err := keyValue(directive, args)
		if err != nil {
			return err
		}
		config.Labels[key] = value
	case "EXEC":
		raw := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		var command []string
		if err := json.Unmarshal([]byte(raw), &command); err != nil {
			return fmt.Errorf("EXEC expects a JSON string array: %w", err)
		}
		config.Exec = command
	default:
		return fmt.Errorf("unknown directive %q", fields[0])
	}

	return nil
}

func singleArg(directive string, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("%s expects exactly one argument", directive)
	}
	return args[0], nil
}

func keyValue(directive string, args []string) (string, string, error) {
	if len(args) != 1 {
		return "", "", fmt.Errorf("%s expects <key>=<value>", directive)
	}
	key, value, ok := strings.Cut(args[0], "=")
	if !ok || key == "" {
		return "", "", fmt.Errorf("%s expects <key>=<value>", directive)
	}
	return key, value, nil
}

// parseMCPLine accepts:
//
//	MCP <name> http  <url>
//	MCP <name> stdio <command> [arg ...]
//
// The legacy two-arg form `MCP <name> <url>` is intentionally rejected — the
// pre-MVP posture allows breaking changes and an explicit transport keyword
// removes the ambiguity between a URL and a stdio command path.
func parseMCPLine(args []string) (agent.MCPServer, error) {
	if len(args) < 3 {
		return agent.MCPServer{}, fmt.Errorf("MCP expects <name> <transport> <url|command> [args...]")
	}
	name := args[0]
	transport := strings.ToLower(args[1])
	switch transport {
	case agent.MCPTransportHTTP:
		if len(args) != 3 {
			return agent.MCPServer{}, fmt.Errorf("MCP %q http expects exactly one URL argument", name)
		}
		return agent.MCPServer{Name: name, Transport: agent.MCPTransportHTTP, URL: args[2]}, nil
	case agent.MCPTransportStdio:
		return agent.MCPServer{
			Name:      name,
			Transport: agent.MCPTransportStdio,
			Command:   args[2],
			Args:      append([]string{}, args[3:]...),
		}, nil
	}
	return agent.MCPServer{}, fmt.Errorf("MCP %q unknown transport %q (want http or stdio)", name, args[1])
}

func parseModel(args []string) (agent.Model, error) {
	if len(args) < 2 {
		return agent.Model{}, fmt.Errorf("MODEL expects <provider> <name> [key=value ...]")
	}

	model := agent.Model{Provider: args[0], Name: args[1]}
	for _, pair := range args[2:] {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return agent.Model{}, fmt.Errorf("MODEL option %q must use key=value", pair)
		}
		switch key {
		case "endpoint":
			model.Endpoint = value
		case "auth":
			model.Auth = value
		case "credential_env":
			model.CredentialEnv = value
		default:
			return agent.Model{}, fmt.Errorf("unknown MODEL option %q", key)
		}
	}
	return model, nil
}

func stripComment(line string) string {
	inString := false
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if char == '#' && !inString {
			return line[:index]
		}
	}
	return line
}
