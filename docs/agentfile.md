# Agentfile

`Agentfile` is a small line-oriented manifest for an agent service. It is intentionally not YAML so `agentctl` can parse it with the Go standard library and keep the grammar explicit.

Blank lines and `#` comments are ignored.

## Directives

```text
AGENT <name>
TYPE <agent-type>
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

## Runtime Contract

The process started by `EXEC` should eventually expose:

- `GET /health`
- `GET /status`
- `GET /tasks`
- `GET /tasks/{task_id}`
- `POST /tasks`
- an MCP endpoint for tool/resource/prompt discovery and invocation

The initial local driver only manages the process lifecycle. Health probing and MCP discovery will be layered on top of the same domain model.
