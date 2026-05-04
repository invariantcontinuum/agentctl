# Agentfile

`Agentfile` is a small line-oriented manifest for an agent service. It is intentionally not YAML so `agentctl` can parse it with the Go standard library and keep the grammar explicit.

Blank lines and `#` comments are ignored.

## Directives

```text
AGENT <name>
IMAGE <agent-image-ref>
TYPE <agent-type>
MODEL <provider> <name> [endpoint=<url>] [auth=<api_key|oauth|api_key_or_oauth|none>] [credential_env=<env-var>]
SKILL <path-or-registry-name>
MCP <name> <url>
VECTOR <name> <provider> <dsn> [collection]
GRAPH <name> <provider> <dsn>
MEMORY <name> <kind> <source>
LOOP <strategy> max_steps=<positive-int>
ENDPOINT <name> <url>
ENV <key>=<value>
LABEL <key>=<value>
EXEC ["program", "arg1", "arg2"]
```

`EXEC` is a JSON array of strings. This keeps command parsing deterministic without adding shell-like quoting rules.

`MODEL` is a provider binding, not provider-specific client logic. Hosted providers such as OpenAI, Anthropic, and Gemini should use `auth=api_key` or `auth=api_key_or_oauth` with credentials referenced by environment variable name. Local providers such as vLLM and llama.cpp should use an OpenAI-compatible local endpoint with `auth=none`.

Examples:

```text
MODEL openai default endpoint=https://api.openai.com/v1 auth=api_key credential_env=OPENAI_API_KEY
MODEL anthropic default endpoint=https://api.anthropic.com auth=api_key credential_env=ANTHROPIC_API_KEY
MODEL gemini default endpoint=https://generativelanguage.googleapis.com auth=api_key_or_oauth credential_env=GEMINI_API_KEY
MODEL vllm local endpoint=http://localhost:8000/v1 auth=none
MODEL llamacpp local endpoint=http://localhost:8102/v1 auth=none
```

## Runtime Contract

The process started by `EXEC` should eventually expose:

- `GET /health`
- `GET /status`
- `GET /tasks`
- `GET /tasks/{task_id}`
- `POST /tasks`
- an MCP endpoint for tool/resource/prompt discovery and invocation

The initial local driver only manages the process lifecycle. Health probing and MCP discovery will be layered on top of the same domain model.
