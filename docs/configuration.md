# Configuration

## State Locations

`agentctl` uses XDG directories through the Go standard library:

- Config state: `${XDG_CONFIG_HOME}/agentctl/state.json`, or the platform default if `XDG_CONFIG_HOME` is unset.
- Logs and traces: `${XDG_CACHE_HOME}/agentctl/`, or the platform default if `XDG_CACHE_HOME` is unset.

Tests and smoke checks should override both directories with temporary paths.

## Agentfile Configuration

`Agentfile` is the canonical manifest for local agent process configuration.

The current directives are documented in [Agentfile](agentfile.md).

## Model Provider Configuration

Model providers are configuration bindings, not CLI-specific clients.

Hosted provider credentials should be referenced by environment variable name:

```text
MODEL openai default endpoint=https://api.openai.com/v1 auth=api_key credential_env=OPENAI_API_KEY
MODEL anthropic default endpoint=https://api.anthropic.com auth=api_key credential_env=ANTHROPIC_API_KEY
MODEL gemini default endpoint=https://generativelanguage.googleapis.com auth=api_key_or_oauth credential_env=GEMINI_API_KEY
```

Local providers use an endpoint and `auth=none`:

```text
MODEL vllm local endpoint=http://localhost:8000/v1 auth=none
MODEL llamacpp local endpoint=http://localhost:8102/v1 auth=none
```

## Global Options Target

The target API includes global defaults:

```text
--loop=react
--model=<provider-or-model>
--mcp=<server>
--debug
--compose
```

These are design targets. They are not all implemented yet.
