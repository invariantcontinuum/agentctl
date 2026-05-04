# What Is Next

This page tracks planned work. It should be updated whenever implementation
lands.

## High Priority

- Live runtime status integration: have `loop ps` query the agent's `/status`
  endpoint instead of only printing recorded configuration.
- Promote `health` reports into `describe` output when an ENDPOINT is set.
- `model` provider config files on disk and provider inspection.
- Stream traces (`agentctl trace -f <id>`) instead of single-shot dump.

## Knowledge

- `rag run "query"` against the agent's MCP retrieval tool.
- `rag trace <id>` filtered to `kind=rag` events.
- Retrieval hit rates and embedding stats sourced from `/status`.

## Action

- Tool latency histograms via `tool trace <agent>`.
- Tool result schema validation against MCP `inputSchema`.

## Persistence

- Live runtime memory inspection via the agent runtime contract; today's
  `memory dump`/`recall` only return configured bindings.

## Control

- Agentfile `GUARD` directive plus `guard ls` populated from configured rules.
- Evaluation and completion criteria.

## Specialization

- Skill registry on disk (durable, queryable).
- Agent image registry beyond the in-memory default catalog.

## Deferred

- Network commands. Do not implement `network ls` or related network surface
  until the network model is explicitly designed.
