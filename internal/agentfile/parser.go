package agentfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

type Parser interface {
	Parse(io.Reader) (agent.Config, error)
}

type LineParser struct{}

func NewParser() LineParser {
	return LineParser{}
}

func (LineParser) Parse(reader io.Reader) (agent.Config, error) {
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
	case "MCP":
		if len(args) != 2 {
			return fmt.Errorf("MCP expects <name> <url>")
		}
		config.MCPServers = append(config.MCPServers, agent.MCPServer{Name: args[0], URL: args[1]})
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
