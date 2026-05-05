# Configuration

## State Locations

`agentctl` uses XDG directories through the Go standard library:

- Instance state: `${XDG_CONFIG_HOME}/agentctl/state.json` (or the platform default).
- Credential store: `${XDG_CONFIG_HOME}/agentctl/credentials.json` (mode `0600`).
- Per-agent runtime config: `${XDG_CACHE_HOME}/agentctl/configs/<id>.json` (mode `0600`; written by `agentctl run` for `agentd` to read back).
- Logs: `${XDG_CACHE_HOME}/agentctl/logs/<id>.log`.
- Traces: `${XDG_CACHE_HOME}/agentctl/traces/<id>.trace`.

Tests and smoke checks should override these directories with temporary
paths.

## Credentials

`agentctl model <provider> auth login` writes per-provider entries to
the credential store. At `run` and `compose up` time the CLI looks up
the agent's `MODEL provider`, copies the API key under
`api_key_env` (or the catalog default — e.g. `ANTHROPIC_API_KEY`),
overrides `MODEL base_url` if the credential record has one, and merges
any `extra_env` switches (such as `CLAUDE_CODE_USE_BEDROCK=1`) into the
child process environment before launch.

## Agentfile Configuration

`Agentfile` is the canonical manifest for local agent process configuration.
It parses into `agent.Config`, whose main children are:

- `Model`: provider, model name, `base_url`, auth mode, API-key env name,
  timeout, and provider-specific options.
- `Skill`: prompt/playbook references or inline content.
- `MCPServer`: HTTP URL/BasePath/Headers or stdio Command/Args/Env, plus
  static tool descriptors.
- `RAGSource`: vector and graph retrieval sources with provider, URL, index,
  labels, weights, and metadata.
- `Memory`: short/long/vector/graph memory bindings with provider, URL or
  bucket, limits, TTL, labels, and metadata.
- `Loop`: loop name, step/token limits, tool-selection policy, hooks,
  evaluation, and multi-agent delegation policy.
- `Endpoint`: structured scheme/host/port/path entries used by health probes
  and default `agentd` address selection.

The current directives are documented in [Agentfile](agentfile.md).

## Model Provider Configuration

Model providers are configuration bindings, not CLI-specific clients.

Hosted provider credentials should be referenced by environment variable name:

```text
MODEL openai default base_url=https://api.openai.com/v1 auth=api_key api_key_env=OPENAI_API_KEY
MODEL anthropic default base_url=https://api.anthropic.com auth=api_key api_key_env=ANTHROPIC_API_KEY
MODEL gemini default base_url=https://generativelanguage.googleapis.com auth=api_key_or_oauth api_key_env=GEMINI_API_KEY
```

Local providers use an endpoint and `auth=none`:

```text
MODEL vllm local base_url=http://localhost:8000/v1 auth=none
MODEL llamacpp local base_url=http://localhost:8102/v1 auth=none
```

## Target Global Options

The target API includes global defaults:

```text
--loop=react
--model=<provider-or-model>
--mcp=<server>
--debug
--compose
```

These are design targets. They are not all implemented yet.
