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
				return agent.Config{}, fmt.Errorf("agentfile:%d: %w", lineNumber, errFromUnsupported)
			}
			if len(fields) != 2 {
				return agent.Config{}, fmt.Errorf("agentfile:%d: FROM expects exactly one path", lineNumber)
			}
			parentPath := fields[1]
			if !filepath.IsAbs(parentPath) {
				parentPath = filepath.Join(filepath.Dir(absolutePath), parentPath)
			}
			parentAbsolute, err := filepath.Abs(parentPath)
			if err != nil {
				return agent.Config{}, fmt.Errorf("agentfile:%d: %w", lineNumber, err)
			}
			parent, err := parseFile(parentAbsolute, visited)
			if err != nil {
				return agent.Config{}, fmt.Errorf("agentfile:%d FROM %s: %w", lineNumber, fields[1], err)
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
			return agent.Config{}, fmt.Errorf("agentfile:%d: %w", lineNumber, err)
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
		skill, err := parseSkill(args)
		if err != nil {
			return err
		}
		config.Skills = append(config.Skills, skill)
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
		source, err := parseRAGSource("vector", args)
		if err != nil {
			return err
		}
		config.VectorStores = append(config.VectorStores, source)
	case "GRAPH":
		source, err := parseRAGSource("graph", args)
		if err != nil {
			return err
		}
		config.GraphStores = append(config.GraphStores, source)
	case "MEMORY":
		memory, err := parseMemory(args)
		if err != nil {
			return err
		}
		config.Memories = append(config.Memories, memory)
	case "LOOP":
		loop, err := parseLoop(args, config.Loop)
		if err != nil {
			return err
		}
		config.Loop = loop
	case "HOOK":
		hook, phase, err := parseHook(args)
		if err != nil {
			return err
		}
		if phase == "pre" {
			config.Loop.PreHooks = append(config.Loop.PreHooks, hook)
		} else {
			config.Loop.PostHooks = append(config.Loop.PostHooks, hook)
		}
	case "EVALUATION":
		evaluation, err := parseEvaluation(args, config.Loop.Evaluation)
		if err != nil {
			return err
		}
		config.Loop.Evaluation = evaluation
	case "MULTI_AGENT":
		multiAgent, err := parseMultiAgent(args, config.Loop.MultiAgent)
		if err != nil {
			return err
		}
		config.Loop.MultiAgent = multiAgent
	case "MCP_TOOL":
		if err := addMCPTool(config, args); err != nil {
			return err
		}
	case "VALIDATOR_TOOL":
		tool, err := parseMCPTool(args)
		if err != nil {
			return err
		}
		config.Loop.Evaluation.ValidatorTools = append(config.Loop.Evaluation.ValidatorTools, tool)
	case "ENDPOINT":
		endpoint, err := parseEndpoint(args)
		if err != nil {
			return err
		}
		config.Endpoints = append(config.Endpoints, endpoint)
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

func parseSkill(args []string) (agent.Skill, error) {
	if len(args) < 1 {
		return agent.Skill{}, fmt.Errorf("SKILL expects <path-or-registry-name> [key=value ...]")
	}
	source := args[0]
	skill := agent.Skill{
		ID:      source,
		Name:    source,
		Path:    source,
		Type:    inferSkillType(source),
		Enabled: true,
	}
	pathSet := false
	for _, pair := range args[1:] {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return agent.Skill{}, fmt.Errorf("SKILL option %q must use key=value", pair)
		}
		switch key {
		case "id":
			skill.ID = value
		case "name":
			skill.Name = value
		case "type":
			skill.Type = value
		case "path":
			skill.Path = value
			pathSet = true
		case "content":
			skill.Content = value
			if !pathSet {
				skill.Path = ""
			}
		case "depends_on", "dependencies":
			skill.Dependencies = splitCSV(value)
		case "enabled":
			enabled, err := strconv.ParseBool(value)
			if err != nil {
				return agent.Skill{}, fmt.Errorf("SKILL enabled must be boolean")
			}
			skill.Enabled = enabled
		default:
			if strings.HasPrefix(key, "metadata.") {
				if skill.Metadata == nil {
					skill.Metadata = map[string]any{}
				}
				skill.Metadata[strings.TrimPrefix(key, "metadata.")] = parseScalar(value)
				continue
			}
			return agent.Skill{}, fmt.Errorf("unknown SKILL option %q", key)
		}
	}
	return skill, nil
}

func inferSkillType(source string) string {
	if strings.HasPrefix(source, "builtin://") {
		return "builtin"
	}
	if strings.HasSuffix(strings.ToLower(source), ".md") {
		return "markdown"
	}
	return "reference"
}

// parseMCPLine accepts:
//
//	MCP <name> http  <url> [key=value ...]
//	MCP <name> stdio <command> [arg ...] [key=value ...]
//
// Both forms populate the same canonical MCPServer descriptor. URL tells the
// agent how to call an already-running server; Command/Args tells agentctl how
// to launch one over stdio or as a future managed subprocess.
func parseMCPLine(args []string) (agent.MCPServer, error) {
	if len(args) < 3 {
		return agent.MCPServer{}, fmt.Errorf("MCP expects <name> <transport> <url|command> [args...]")
	}
	name := args[0]
	transport := strings.ToLower(args[1])
	server := agent.MCPServer{Name: name, Enabled: true}
	switch transport {
	case "http":
		server.URL = args[2]
		if err := applyMCPOptions(&server, args[3:]); err != nil {
			return agent.MCPServer{}, err
		}
		return server, nil
	case "stdio":
		server.Command = args[2]
		if err := applyMCPArgsAndOptions(&server, args[3:]); err != nil {
			return agent.MCPServer{}, err
		}
		return server, nil
	}
	return agent.MCPServer{}, fmt.Errorf("MCP %q unknown transport %q (want http or stdio)", name, args[1])
}

func applyMCPArgsAndOptions(server *agent.MCPServer, args []string) error {
	for _, value := range args {
		if isMCPOption(value) {
			if err := applyMCPOption(server, value); err != nil {
				return err
			}
			continue
		}
		server.Args = append(server.Args, value)
	}
	return nil
}

func applyMCPOptions(server *agent.MCPServer, args []string) error {
	for _, value := range args {
		if !isMCPOption(value) {
			return fmt.Errorf("MCP %q http option %q must use key=value", server.Name, value)
		}
		if err := applyMCPOption(server, value); err != nil {
			return err
		}
	}
	return nil
}

func isMCPOption(value string) bool {
	key, _, ok := strings.Cut(value, "=")
	if !ok {
		return false
	}
	return key == "url" ||
		key == "base_path" ||
		key == "timeout_sec" ||
		key == "enabled" ||
		strings.HasPrefix(key, "header.") ||
		strings.HasPrefix(key, "env.")
}

func applyMCPOption(server *agent.MCPServer, pair string) error {
	key, value, _ := strings.Cut(pair, "=")
	switch {
	case key == "url":
		server.URL = value
	case key == "base_path":
		server.BasePath = value
	case key == "timeout_sec":
		timeout, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("MCP %q timeout_sec must be an integer", server.Name)
		}
		server.TimeoutSec = timeout
	case key == "enabled":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("MCP %q enabled must be boolean", server.Name)
		}
		server.Enabled = enabled
	case strings.HasPrefix(key, "header."):
		if server.Headers == nil {
			server.Headers = map[string]string{}
		}
		server.Headers[strings.TrimPrefix(key, "header.")] = value
	case strings.HasPrefix(key, "env."):
		if server.Env == nil {
			server.Env = map[string]string{}
		}
		server.Env[strings.TrimPrefix(key, "env.")] = value
	default:
		return fmt.Errorf("unknown MCP option %q", key)
	}
	return nil
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
		case "base_url":
			model.BaseURL = value
		case "api_key_env":
			model.APIKeyEnv = value
		case "auth":
			model.Auth = value
		case "timeout_sec":
			timeout, err := strconv.Atoi(value)
			if err != nil {
				return agent.Model{}, fmt.Errorf("MODEL timeout_sec must be an integer")
			}
			model.TimeoutSec = timeout
		default:
			if strings.HasPrefix(key, "option.") {
				if model.Options == nil {
					model.Options = map[string]any{}
				}
				model.Options[strings.TrimPrefix(key, "option.")] = parseScalar(value)
				continue
			}
			return agent.Model{}, fmt.Errorf("unknown MODEL option %q", key)
		}
	}
	return model, nil
}

func parseRAGSource(sourceType string, args []string) (agent.RAGSource, error) {
	if len(args) < 3 {
		return agent.RAGSource{}, fmt.Errorf("%s expects <name> <provider> <url> [index] [key=value ...]", strings.ToUpper(sourceType))
	}
	source := agent.RAGSource{
		ID:       args[0],
		Name:     args[0],
		Type:     sourceType,
		Provider: args[1],
		URL:      args[2],
	}
	rest := args[3:]
	if len(rest) > 0 && !strings.Contains(rest[0], "=") {
		source.Index = rest[0]
		rest = rest[1:]
	}
	for _, pair := range rest {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return agent.RAGSource{}, fmt.Errorf("%s option %q must use key=value", strings.ToUpper(sourceType), pair)
		}
		switch key {
		case "id":
			source.ID = value
		case "name":
			source.Name = value
		case "type":
			source.Type = value
		case "url":
			source.URL = value
		case "index":
			source.Index = value
		case "embedding_model":
			source.EmbeddingModel = value
		case "weight":
			weight, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return agent.RAGSource{}, fmt.Errorf("%s weight must be a number", strings.ToUpper(sourceType))
			}
			source.Weight = weight
		default:
			if strings.HasPrefix(key, "label.") {
				if source.Labels == nil {
					source.Labels = map[string]string{}
				}
				source.Labels[strings.TrimPrefix(key, "label.")] = value
				continue
			}
			if strings.HasPrefix(key, "metadata.") {
				if source.Metadata == nil {
					source.Metadata = map[string]any{}
				}
				source.Metadata[strings.TrimPrefix(key, "metadata.")] = parseScalar(value)
				continue
			}
			return agent.RAGSource{}, fmt.Errorf("unknown %s option %q", strings.ToUpper(sourceType), key)
		}
	}
	return source, nil
}

func parseMemory(args []string) (agent.Memory, error) {
	if len(args) < 3 {
		return agent.Memory{}, fmt.Errorf("MEMORY expects <name> <type> <provider> [url-or-bucket] [key=value ...]")
	}
	memory := agent.Memory{
		ID:       args[0],
		Name:     args[0],
		Type:     args[1],
		Provider: args[2],
	}
	rest := args[3:]
	if len(rest) > 0 && !strings.Contains(rest[0], "=") {
		applyMemoryLocation(&memory, rest[0])
		rest = rest[1:]
	}
	for _, pair := range rest {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return agent.Memory{}, fmt.Errorf("MEMORY option %q must use key=value", pair)
		}
		if err := applyMemoryOption(&memory, key, value); err != nil {
			return agent.Memory{}, err
		}
	}
	return memory, nil
}

func applyMemoryLocation(memory *agent.Memory, location string) {
	if strings.Contains(location, "://") {
		memory.URL = location
		return
	}
	memory.Bucket = location
}

func applyMemoryOption(memory *agent.Memory, key string, value string) error {
	switch key {
	case "id":
		memory.ID = value
	case "name":
		memory.Name = value
	case "type":
		memory.Type = value
	case "provider":
		memory.Provider = value
	case "url":
		memory.URL = value
	case "bucket":
		memory.Bucket = value
	case "limit":
		limit, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("MEMORY %s must be an integer", key)
		}
		memory.Limit = limit
	case "ttl_sec":
		ttl, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("MEMORY ttl_sec must be an integer")
		}
		memory.TTLSec = ttl
	default:
		if strings.HasPrefix(key, "label.") {
			if memory.Labels == nil {
				memory.Labels = map[string]string{}
			}
			memory.Labels[strings.TrimPrefix(key, "label.")] = value
			return nil
		}
		if strings.HasPrefix(key, "metadata.") {
			if memory.Metadata == nil {
				memory.Metadata = map[string]any{}
			}
			memory.Metadata[strings.TrimPrefix(key, "metadata.")] = parseScalar(value)
			return nil
		}
		return fmt.Errorf("unknown MEMORY option %q", key)
	}
	return nil
}

func parseLoop(args []string, current agent.Loop) (agent.Loop, error) {
	if len(args) < 2 {
		return agent.Loop{}, fmt.Errorf("LOOP expects <name> max_steps=<positive-int> [key=value ...]")
	}
	current.Name = args[0]
	for _, pair := range args[1:] {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return agent.Loop{}, fmt.Errorf("LOOP option %q must use key=value", pair)
		}
		switch key {
		case "max_steps":
			maxSteps, err := strconv.Atoi(value)
			if err != nil {
				return agent.Loop{}, fmt.Errorf("LOOP max_steps must be an integer")
			}
			current.MaxSteps = maxSteps
		case "max_tokens":
			maxTokens, err := strconv.Atoi(value)
			if err != nil {
				return agent.Loop{}, fmt.Errorf("LOOP max_tokens must be an integer")
			}
			current.MaxTokens = maxTokens
		case "tool_selection":
			current.ToolSelection = value
		default:
			return agent.Loop{}, fmt.Errorf("unknown LOOP option %q", key)
		}
	}
	return current, nil
}

func parseHook(args []string) (agent.Hook, string, error) {
	if len(args) < 3 {
		return agent.Hook{}, "", fmt.Errorf("HOOK expects <pre|post> <name> <type> [key=value ...]")
	}
	phase := strings.ToLower(args[0])
	if phase != "pre" && phase != "post" {
		return agent.Hook{}, "", fmt.Errorf("HOOK phase must be pre or post")
	}
	hook := agent.Hook{Name: args[1], Type: args[2]}
	for _, pair := range args[3:] {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return agent.Hook{}, "", fmt.Errorf("HOOK option %q must use key=value", pair)
		}
		switch key {
		case "url":
			hook.URL = value
		case "timeout_sec":
			timeout, err := strconv.Atoi(value)
			if err != nil {
				return agent.Hook{}, "", fmt.Errorf("HOOK timeout_sec must be an integer")
			}
			hook.TimeoutSec = timeout
		case "on_error":
			hook.OnError = value
		default:
			if strings.HasPrefix(key, "header.") {
				if hook.Headers == nil {
					hook.Headers = map[string]string{}
				}
				hook.Headers[strings.TrimPrefix(key, "header.")] = value
				continue
			}
			if strings.HasPrefix(key, "label.") {
				if hook.Labels == nil {
					hook.Labels = map[string]string{}
				}
				hook.Labels[strings.TrimPrefix(key, "label.")] = value
				continue
			}
			return agent.Hook{}, "", fmt.Errorf("unknown HOOK option %q", key)
		}
	}
	return hook, phase, nil
}

func parseEvaluation(args []string, current agent.Evaluation) (agent.Evaluation, error) {
	for _, pair := range args {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return agent.Evaluation{}, fmt.Errorf("EVALUATION option %q must use key=value", pair)
		}
		switch key {
		case "max_errors":
			maxErrors, err := strconv.Atoi(value)
			if err != nil {
				return agent.Evaluation{}, fmt.Errorf("EVALUATION max_errors must be an integer")
			}
			current.MaxErrors = maxErrors
		case "tool_allow_list":
			current.ToolAllowList = splitCSV(value)
		case "tool_deny_list":
			current.ToolDenyList = splitCSV(value)
		case "log_filter":
			current.LogFilter = splitCSV(value)
		case "completion_criteria":
			current.CompletionCriteria = splitCSV(value)
		default:
			return agent.Evaluation{}, fmt.Errorf("unknown EVALUATION option %q", key)
		}
	}
	return current, nil
}

func parseMultiAgent(args []string, current agent.MultiAgentConfig) (agent.MultiAgentConfig, error) {
	for _, pair := range args {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return agent.MultiAgentConfig{}, fmt.Errorf("MULTI_AGENT option %q must use key=value", pair)
		}
		switch key {
		case "enabled":
			enabled, err := strconv.ParseBool(value)
			if err != nil {
				return agent.MultiAgentConfig{}, fmt.Errorf("MULTI_AGENT enabled must be boolean")
			}
			current.Enabled = enabled
		case "coordinator":
			current.Coordinator = value
		case "allowed_roles":
			current.AllowedRoles = splitCSV(value)
		case "delegation":
			current.Delegation = value
		default:
			if strings.HasPrefix(key, "policy.") {
				if current.Policy == nil {
					current.Policy = map[string]any{}
				}
				current.Policy[strings.TrimPrefix(key, "policy.")] = parseScalar(value)
				continue
			}
			return agent.MultiAgentConfig{}, fmt.Errorf("unknown MULTI_AGENT option %q", key)
		}
	}
	return current, nil
}

func addMCPTool(config *agent.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("MCP_TOOL expects <server> <id> <name> [key=value ...]")
	}
	serverName := args[0]
	tool, err := parseMCPTool(args[1:])
	if err != nil {
		return err
	}
	for index := range config.MCPServers {
		if config.MCPServers[index].Name == serverName {
			config.MCPServers[index].Tools = append(config.MCPServers[index].Tools, tool)
			return nil
		}
	}
	return fmt.Errorf("MCP_TOOL references unknown server %q", serverName)
}

func parseMCPTool(args []string) (agent.MCPTool, error) {
	if len(args) < 2 {
		return agent.MCPTool{}, fmt.Errorf("tool expects <id> <name> [key=value ...]")
	}
	tool := agent.MCPTool{ID: args[0], Name: args[1], Enabled: true}
	for _, pair := range args[2:] {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return agent.MCPTool{}, fmt.Errorf("tool option %q must use key=value", pair)
		}
		switch key {
		case "description":
			tool.Description = value
		case "category":
			tool.Category = value
		case "enabled":
			enabled, err := strconv.ParseBool(value)
			if err != nil {
				return agent.MCPTool{}, fmt.Errorf("tool enabled must be boolean")
			}
			tool.Enabled = enabled
		default:
			if strings.HasPrefix(key, "metadata.") {
				if tool.Metadata == nil {
					tool.Metadata = map[string]any{}
				}
				tool.Metadata[strings.TrimPrefix(key, "metadata.")] = parseScalar(value)
				continue
			}
			return agent.MCPTool{}, fmt.Errorf("unknown tool option %q", key)
		}
	}
	return tool, nil
}

func parseEndpoint(args []string) (agent.Endpoint, error) {
	if len(args) < 2 {
		return agent.Endpoint{}, fmt.Errorf("ENDPOINT expects <name> <url> or <name> scheme=<scheme> host=<host> [port=N] [path=/x]")
	}
	name := args[0]
	if !strings.Contains(args[1], "=") {
		endpoint, err := agent.EndpointFromURL(name, args[1])
		if err != nil {
			return agent.Endpoint{}, err
		}
		return endpoint, nil
	}
	endpoint := agent.Endpoint{Name: name}
	for _, pair := range args[1:] {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return agent.Endpoint{}, fmt.Errorf("ENDPOINT option %q must use key=value", pair)
		}
		switch key {
		case "scheme":
			endpoint.Scheme = value
		case "host":
			endpoint.Host = value
		case "port":
			port, err := strconv.Atoi(value)
			if err != nil {
				return agent.Endpoint{}, fmt.Errorf("ENDPOINT port must be an integer")
			}
			endpoint.Port = port
		case "path":
			endpoint.Path = value
		default:
			if strings.HasPrefix(key, "label.") {
				if endpoint.Labels == nil {
					endpoint.Labels = map[string]string{}
				}
				endpoint.Labels[strings.TrimPrefix(key, "label.")] = value
				continue
			}
			return agent.Endpoint{}, fmt.Errorf("unknown ENDPOINT option %q", key)
		}
	}
	return endpoint, nil
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseScalar(value string) any {
	if parsed, err := strconv.ParseBool(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return parsed
	}
	return value
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
