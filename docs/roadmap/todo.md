# What Is Next

This page tracks planned work. It should be updated whenever implementation lands.

## High Priority

- Add `agent` singular management namespace while preserving Docker-like top-level aliases.
- Add `exec` for running a tool or step in an agent context.
- Add structured trace events for plan, RAG, tool, memory, guard, reflection, and delegation.
- Add runtime health probing for `/health`, `/status`, and `/tasks`.
- Add MCP discovery for tool schemas.
- Add model provider config files and provider inspection.

## Knowledge

- `rag ls`
- `rag vector ls`
- `rag graph ls`
- `rag run "query"`
- `rag trace <id>`
- Retrieval hit rates and embedding stats.

## Action

- `tool ls`
- `tool mcp ls`
- `tool exec <agent> <tool> ...`
- Tool latency and tool result trace.

## Persistence

- `memory ls`
- `memory short ls`
- `memory long ls`
- `memory dump <agent>`
- `memory recall <agent> "key"`

## Control

- `loop ls`
- `loop ps <agent>`
- `loop trace <agent>`
- `guard ls <agent>`
- Evaluation and completion criteria.

## Specialization

- Skill registry.
- Agent image registry.
- Multi-agent teams.
- `agent compose up`.

## Deferred

- Network commands. Do not implement `network ls` or related network surface until the network model is explicitly designed.
