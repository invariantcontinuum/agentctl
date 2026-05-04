# Architecture

`agentctl` is organized around small, single-purpose Go packages. The
control plane (`cmd/agentctl`) and the runtime (`cmd/agentd`) are
separate binaries that share the `internal/` libraries.

## Binaries

- `cmd/agentctl` — Docker-style CLI: parses Agentfiles / AgentCompose,
  manages local state, and starts the runtime.
- `cmd/agentd` — bundled agent runtime that hosts a single agent
  instance behind the runtime contract (`/health`, `/status`, `/tasks`,
  `POST /tasks`, `/tasks/{id}`). `agentctl run` defaults `EXEC` to
  `agentd --config <path> --addr <host:port>` whenever the Agentfile
  doesn't supply its own command.

## Packages

- `internal/cli` — command parsing, output formatting, credential and
  default-EXEC injection at run time.
- `internal/agent` — domain types (`Config`, `Model`, `Skill`,
  `MCPServer`, `RAGSource`, `Memory`, `Loop`, `Endpoint`, ...), validation,
  and endpoint URL/address helpers.
- `internal/agentfile` — line-oriented manifest parser, including
  Docker-like `FROM <relative-path>` inheritance with
  override-on-singletons / append-on-lists / merge-on-maps semantics
  and cycle detection.
- `internal/agentsdk` — provider-neutral agent runtime mirroring the
  Anthropic Agent SDK, OpenAI Agents SDK, and Google ADK-Go patterns.
  Houses `Agent`, `Runnable`, `ModelClient`, `Tool`, `Session`,
  `Hooks`, `Guardrail`, the multi-agent orchestrators (`Sequential`,
  `Parallel`, `Loop`, `Handoff`, `Isolated`), and concrete provider
  clients (`AnthropicClient`, `OpenAIClient`, `GeminiClient`,
  `EchoClient`). See [Agent SDK](agent-sdk.md).
- `internal/runtime` — `agentd`'s in-process implementation: bounded
  task `Store` (FIFO eviction of terminal tasks), HTTP server with
  per-route handlers, MCP tool discovery on boot, and a single worker
  goroutine that drives the agent loop per submitted task.
- `internal/credentials` — file-backed JSON credential store at
  `${XDG_CONFIG_HOME}/agentctl/credentials.json` (mode 0600) with
  per-provider records (api_key, endpoint, extra_env).
- `internal/mcp` — Model Context Protocol client (`tools/list`,
  `tools/call`) for both `http` and `stdio` transports.
- `internal/compose` — `AgentCompose` parser plus topological `Plan()`
  using Kahn's algorithm with a min-heap by name; equal-rank services
  are emitted alphabetically.
- `internal/health` — runtime-contract probe used by `agentctl health`
  and `compose up`'s readiness gate.
- `internal/logging` — structured JSON-Lines logger (`debug` / `info`
  / `warn` / `error`) and the `agentctl logs --level <L>` filter.
- `internal/trace` — typed structured trace events written next to
  each agent's log file (`run`, `start`, `stop`, `rm`, `tool`,
  `health`, plus reserved kinds for planning, RAG, memory, guard,
  reflection, delegation).
- `internal/catalog` — role image catalog (planner / coder /
  researcher / reviewer / executor / coordinator).
- `internal/model` — model provider catalog backing
  `agentctl model ls` and the credential UX.
- `internal/driver` — runtime driver interface and local process
  driver (Linux/macOS via `os/exec` + signals; Windows variants).
- `internal/store` — JSON-backed local instance state at
  `${XDG_CONFIG_HOME}/agentctl/state.json`.

## Design Rules

- Keep command handlers thin: routing + formatting only.
- Put lifecycle behavior behind `driver.Driver`.
- Put role specialization behind the image catalog and provider
  details behind the model catalog.
- Drive every agent through `agentsdk` interfaces (`Runnable`,
  `ModelClient`, `Tool`, `Session`); never branch on provider in the
  agent loop.
- Keep credential storage and injection behind `internal/credentials`;
  the CLI reads, the runtime never writes.
- Keep `inspect` machine-readable and `describe` human-readable.
- Tunables (queue capacity, retention, timeouts) live in
  `runtime.Options`, not module-level constants.

## Runtime in Practice

Today, `cmd/agentd` already coordinates:

- Agent lifecycle via the bounded task store.
- Planning loops via `agentsdk.Agent.Run` (system + tools + hooks +
  guardrails).
- MCP tool calling via `agentsdk.DiscoverMCPTools` against each
  Agentfile `MCP` directive.
- Multi-agent orchestration via `Sequential`/`Parallel`/`Loop`/`Handoff`
  agents (composable via the `Runnable` interface).
- `compose up` health-gates each service against `/health` before
  starting the next service in topological order.

The pieces that remain target-state — vector/graph/hybrid RAG, short
and long-term memory, guardrail evaluation, delegation across
processes — are tracked in [Roadmap → What Is Next](../roadmap/todo.md).

Each component exposes Docker-style operations: `run`, `ls` / `ps`,
`logs`, `exec`, `trace`, `inspect`, `describe`.
