# What Is Next

## High Priority

- `agentctl session` group mirroring the Anthropic Agent SDK and OpenAI
  Agents SDK session shapes (resume, fork, dump, list). The `agentsdk.FileSession`
  storage exists; the CLI surface does not.
- Live `loop ps` integration with the agent's `/status` endpoint.
- Promote `health` reports into `describe` output when an ENDPOINT is set.
- Stream traces and logs (`agentctl trace -f <id>`, `agentctl logs -f <id>`).

## Knowledge

- `rag run "query"` against the agent's MCP retrieval tool.
- `rag trace <id>` filtered to `kind=rag` events.
- Retrieval hit rates and embedding stats sourced from `/status`.

## Action

- Tool latency histograms via `tool trace <agent>`.
- Tool result schema validation against MCP `inputSchema`.
- Bedrock / Vertex / Foundry credential helpers (agent SDK switches).

## Persistence

- Live runtime memory inspection via the agent runtime contract.

## Control

- Agentfile `GUARD` directive plus `guard ls` populated from configured rules.
- Evaluation and completion criteria.

## Specialization

- Skill registry on disk (durable, queryable).
- Agent image registry beyond the in-memory default catalog.

## Deferred

- Network commands. Do not implement `network ls` or related network surface
  until the network model is explicitly designed.
