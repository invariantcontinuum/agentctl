# Agentfile

`Agentfile` is a small line-oriented manifest for an agent service. It is
intentionally not YAML so `agentctl` can parse it with the Go standard library
and keep the grammar explicit.

Blank lines and `#` comments are ignored.

## Directives

```text
FROM <parent-agentfile-path>
AGENT <name>
IMAGE <agent-image-ref>
TYPE <agent-type>
MODEL <provider> <name> [endpoint=<url>] [auth=<api_key|oauth|api_key_or_oauth|none>] [credential_env=<env-var>]
SKILL <path-or-registry-name>
MCP <name> http  <url>
MCP <name> stdio <command> [arg ...]
VECTOR <name> <provider> <dsn> [collection]
GRAPH <name> <provider> <dsn>
MEMORY <name> <kind> <source>
LOOP <strategy> max_steps=<positive-int>
ENDPOINT <name> <url>
ENV <key>=<value>
LABEL <key>=<value>
EXEC ["program", "arg1", "arg2"]
```

`EXEC` is a JSON array of strings. This keeps command parsing deterministic
without adding shell-like quoting rules.

## FROM (Docker-like inheritance)

Agentfiles compose the same way Dockerfiles do. A child Agentfile that begins
with

```text
FROM ./base/Agentfile
```

inherits every directive from the parent before its own directives are
applied. Merge semantics:

- Single-value directives (`AGENT`, `IMAGE`, `TYPE`, `MODEL`, `LOOP`, `EXEC`)
  in the child override the parent.
- List directives (`SKILL`, `MCP`, `VECTOR`, `GRAPH`, `MEMORY`, `ENDPOINT`)
  in the child append to the parent's list.
- Map directives (`ENV`, `LABEL`) in the child merge with the parent. Child
  wins on key conflict.

`FROM` is resolved relative to the file that contains it. Cycles are detected
and rejected. `FROM` is only valid via on-disk parsing — streaming `Parse(r)`
calls reject it because there's no path to resolve against.

See `examples/base/Agentfile` and `examples/from-base/Agentfile` for a working
pair.

## MCP transports

`MCP` accepts either an HTTP endpoint or a stdio child process, matching the
Anthropic Agent SDK and OpenAI Agents SDK MCP shapes.

```text
MCP search http  http://localhost:9001/mcp
MCP fs     stdio npx -y @modelcontextprotocol/server-filesystem /tmp
```

The transport keyword (`http` or `stdio`) is required. `agentctl tool mcp ls`
discovers tools over both transports; `agentctl tool exec` invokes
`tools/call` over the matching transport.

## MODEL bindings

`MODEL` is a provider binding, not provider-specific client logic. Hosted
providers such as OpenAI, Anthropic, and Gemini should use `auth=api_key` or
`auth=api_key_or_oauth` with credentials referenced by environment variable
name. Local providers such as vLLM and llama.cpp should use an
OpenAI-compatible local endpoint with `auth=none`.

API keys can also be persisted with `agentctl model <provider> auth login`;
see [Models and Auth](models.md).

```text
MODEL openai default endpoint=https://api.openai.com/v1 auth=api_key credential_env=OPENAI_API_KEY
MODEL anthropic default endpoint=https://api.anthropic.com auth=api_key credential_env=ANTHROPIC_API_KEY
MODEL gemini default endpoint=https://generativelanguage.googleapis.com auth=api_key_or_oauth credential_env=GEMINI_API_KEY
MODEL vllm local endpoint=http://localhost:8000/v1 auth=none
MODEL llamacpp local endpoint=http://localhost:8102/v1 auth=none
```

## Runtime contract

The process started by `EXEC` should eventually expose:

- `GET /health`
- `GET /status`
- `GET /tasks`
- `GET /tasks/{task_id}`
- `POST /tasks`
- an MCP endpoint for tool/resource/prompt discovery and invocation

The agent process should also write JSON-Lines log records of the shape

```json
{"ts":"2026-05-04T10:00:00Z","level":"info","msg":"started","fields":{"task":"123"}}
```

so `agentctl logs --level <level> <id>` can filter them. Lines that aren't
valid JSON are still printed as info-level lines, so non-conforming agents
remain readable.
