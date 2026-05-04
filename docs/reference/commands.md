# Implemented Commands

## Top-Level Commands

| Command | Status | Purpose |
| --- | --- | --- |
| `run` | Implemented | Start an agent from an `Agentfile` or role image. |
| `ps` | Implemented | List running agents by default; `-a` includes stopped agents; `-q` prints IDs only. |
| `logs` | Implemented | Print local process log file. |
| `trace` | Implemented | Print local lifecycle trace file. |
| `inspect` | Implemented | Print JSON state. |
| `describe` | Implemented | Print human-readable state. |
| `stop` | Implemented | Stop an agent process. |
| `start` | Implemented | Start a stopped agent from recorded config. |
| `restart` | Implemented | Stop and start an agent. |
| `rm` | Implemented | Remove stopped agent state; `-f` also stops running agents. |

## Grouped Commands

| Command | Status | Purpose |
| --- | --- | --- |
| `agents ls` | Implemented | Grouped form of `ps`. |
| `agents describe` | Implemented | Grouped form of `describe`. |
| `agents rm` | Implemented | Grouped form of `rm`. |
| `models ls` | Implemented | List model provider definitions. |
| `skills ls` | Implemented | List local skill files/directories. |
| `tools ls` | Implemented | List configured MCP server endpoints for an agent. |

## Examples

```bash
agentctl run --rm coder:latest
agentctl ps -aq
agentctl agents ls
agentctl models ls
agentctl describe coder-<suffix>
agentctl rm -f coder-<suffix>
```
