# What Exists Now

## Runtime

- Local process driver.
- JSON-backed local state.
- XDG config/cache state locations.
- Structured JSON-Lines lifecycle and reasoning trace file (`run`, `start`,
  `stop`, `rm`, `tool`, `health`, plus reserved kinds for `plan`, `rag`,
  `memory`, `guard`, `reflection`, `delegation`).
- Bundled `agentd` runtime binary that hosts a single agent and exposes the
  runtime contract (`/health`, `/status`, `/tasks`, `POST /tasks`,
  `/tasks/{id}`). `agentctl run` defaults `EXEC` to `agentd --config <path> --addr <host:port>` whenever the Agentfile omits it.
- `internal/agentsdk` — provider-neutral agent runtime mirroring the
  Anthropic Agent SDK, OpenAI Agents SDK, and Google ADK-Go patterns:
  `Agent` (single model + tools loop), `SequentialAgent`, `ParallelAgent`,
  `LoopAgent`, `HandoffAgent`, `IsolatedAgent`. Hooks (BeforeRun, AfterRun,
  BeforeTool, AfterTool) and Guardrails (RegexGuard, MaxLengthGuard) are
  optional injection points. Sessions are pluggable: `MemorySession` and
  `FileSession` (JSON-Lines).
- Built-in model clients with tool-use round-trip: Anthropic Messages
  (`tool_use` blocks), OpenAI Chat Completions (`function tools` —
  works for vLLM and llama.cpp too), Google Gemini generateContent
  (`functionCall`/`functionResponse`), and deterministic Echo fallback.
- MCP tool adapter (`agentsdk.DiscoverMCPTools`) wraps each remote tool
  exposed by an Agentfile `MCP` server as a callable Tool the Agent can
  invoke.
- `compose up` health-gates each service against `/health` before starting
  the next service in topological order.

## Agent Manifests

- Line-oriented `Agentfile`.
- Docker-like `FROM <parent-Agentfile>` inheritance with override (single-value)
  and append (list-value) merge semantics. Cycles are rejected.
- `IMAGE`, `AGENT`, `TYPE`, `MODEL`, `SKILL`, `MCP`, `MCP_TOOL`, `VECTOR`,
  `GRAPH`, `MEMORY`, `LOOP`, `HOOK`, `EVALUATION`, `VALIDATOR_TOOL`,
  `MULTI_AGENT`, `ENDPOINT`, `ENV`, `LABEL`, and `EXEC` directives.
- `MCP <name> http <url>` and `MCP <name> stdio <command> [args]` transports
  (matching the Anthropic Agent SDK / OpenAI Agents SDK MCP shapes).

## Multi-Agent Compose

- Line-oriented `AgentCompose` document with `COMPOSE` and `AGENT` directives.
- `agentctl compose ls`, `compose up`, `compose down`, `compose ps`.
- Topological start order from `DEPENDS_ON`.

## Auth and Credentials

- `model <provider> auth login|logout|status` plus `model auth ls`.
- File-backed credentials store at `${XDG_CONFIG_HOME}/agentctl/credentials.json`
  (mode 0600).
- Interactive prompt with POSIX `stty -echo` masking when available; falls
  back to visible echo with a notice on platforms where `stty` is missing.
- `model ls` shows a LOGGED IN column.
- `agentctl run` and `compose up` auto-inject the API key (under the
  catalog-backed `api_key_env`) plus any `extra_env` switches into the
  child process before launch.

## Structured Logging

- `internal/logging` package with debug/info/warn/error levels and JSON-Lines
  records.
- `agentctl logs --level <level>` filters the agent's log file. Non-JSON
  lines surface as info so legacy or non-conforming agents stay readable.
- `--json` re-emits records verbatim for downstream tooling.

## Docker-Like UX

- `run`, `ps -a/-q/-aq`, `logs --level/--json`, `trace`, `inspect`,
  `describe`, `stop`, `start`, `restart`, `rm -f`.
- Singular and plural noun groups: `agent | agents`, `model | models`,
  `skill | skills`, `tool | tools`.

## Knowledge / Action / Persistence / Control

- `rag ls`, `rag vector ls`, `rag graph ls`.
- `tool ls`, `tool mcp ls`, `tool exec`, top-level `exec` alias.
- `memory ls`, `memory short ls`, `memory long ls`, `memory dump`,
  `memory recall`.
- `loop ls`, `loop ps`, `loop trace`.
- `guard ls` (placeholder; reserved for future GUARD directive).

## Health & Discovery

- `agentctl health <id>` probes `/health`, `/status`, `/tasks` and records a
  `health` trace event.
- `agentctl tool mcp ls <id>` discovers tool schemas via JSON-RPC (http or
  stdio).
- `agentctl tool exec <id> <tool>` invokes `tools/call` and records a `tool`
  trace event with latency and status.

## Catalogs

- Role images: planner, researcher, coder, reviewer, executor, coordinator.
- Model providers: OpenAI, Anthropic, Gemini, vLLM, llama.cpp.

## CI and Release

- Formatting, vet, test, build.
- Syft SBOM workflow.
- Grype CVE scan workflow.
- GitHub Pages documentation deployment workflow for the Squidfunk Material for MkDocs site.
- Release workflow for archives, `.deb`, `.rpm`, checksums, and release SBOM.
