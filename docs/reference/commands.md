# Implemented Commands

## Lifecycle

| Command | Purpose |
| --- | --- |
| `run` | Start an agent from an `Agentfile` or role image. |
| `ps` | List running agents by default; `-a` includes stopped agents; `-q` prints IDs only. |
| `logs <id>` | Print local process log file. |
| `trace <id>` | Print structured JSON-Lines trace events; `--json` keeps the raw payload. |
| `inspect <id>` | Print JSON state. |
| `describe <id>` | Print human-readable state. |
| `stop <id>` / `start <id>` / `restart <id>` | Process lifecycle. |
| `rm [-f] <id>` | Remove stopped agent state; `-f` also stops running agents. |

## Compose

| Command | Purpose |
| --- | --- |
| `compose ls [-f path]` | List compose services in topological order. |
| `compose up [-f path] [--dry-run]` | Start every AGENT in dependency order. |
| `compose down [-f path]` | Stop and remove every agent labelled with the compose project. |
| `compose ps [-f path]` | List running compose services. |

## Action

| Command | Purpose |
| --- | --- |
| `tool ls <id>` | List configured MCP servers for an agent. |
| `tool mcp ls <id>` | Discover tool schemas via `tools/list` against each MCP server. |
| `tool exec [--server NAME] [--args JSON] <id> <tool>` | Invoke `tools/call` on the chosen MCP server. |
| `exec [--server NAME] [--args JSON] <id> <tool>` | Top-level alias for `tool exec`. |

## Knowledge

| Command | Purpose |
| --- | --- |
| `rag ls <id>` | Print VECTOR + GRAPH RAG sources. |
| `rag vector ls <id>` | Print VECTOR RAG sources only. |
| `rag graph ls <id>` | Print GRAPH RAG sources only. |

## Persistence

| Command | Purpose |
| --- | --- |
| `memory ls <id>` | List configured memory bindings. |
| `memory short ls <id>` | Filter to `kind=short` bindings. |
| `memory long ls <id>` | Filter to `kind=long` bindings. |
| `memory dump <id>` | Print memory bindings as JSON. |
| `memory recall <id> <key>` | Print the binding metadata for `<key>`. |

## Control

| Command | Purpose |
| --- | --- |
| `loop ls` | List loop strategy and limits per agent. |
| `loop ps <id>` | Print loop summary for one agent. |
| `loop trace <id>` | Print the trace file scoped by client-side filtering. |
| `guard ls <id>` | Placeholder; surfaces the reserved GUARD directive. |

## Health

| Command | Purpose |
| --- | --- |
| `health [--url URL] [--json] <id>` | Probe `/health`, `/status`, `/tasks`; record a `health` trace event. |

## Management Aliases

| Command | Purpose |
| --- | --- |
| `agent ls` / `agents ls` | Equivalent to `ps`. |
| `agent describe` / `agents describe` | Equivalent to `describe`. |
| `agent rm` / `agents rm` | Equivalent to `rm`. |
| `model ls` / `models ls` | List model provider definitions. |
| `skill ls` / `skills ls` | List local skill files/directories. |
| `tool ls` / `tools ls` | List configured MCP server endpoints for an agent. |

## Examples

```bash
agentctl run --rm coder:latest
agentctl ps -aq
agentctl agent ls
agentctl model ls
agentctl describe coder-<suffix>
agentctl rm -f coder-<suffix>

agentctl compose up -f examples/team/AgentCompose
agentctl compose ps -f examples/team/AgentCompose
agentctl compose down -f examples/team/AgentCompose

agentctl tool mcp ls coder-<suffix>
agentctl exec --args '{"q":"agents"}' coder-<suffix> search

agentctl health planner-<suffix>
agentctl trace planner-<suffix>
```
