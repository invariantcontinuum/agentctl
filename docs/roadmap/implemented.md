# What Exists Now

## Runtime

- Local process driver.
- JSON-backed local state.
- XDG config/cache state locations.
- Structured JSON-Lines lifecycle and reasoning trace file (`run`, `start`,
  `stop`, `rm`, `tool`, `health`, plus reserved kinds for `plan`, `rag`,
  `memory`, `guard`, `reflection`, `delegation`).

## Agent Manifests

- Line-oriented `Agentfile`.
- `IMAGE`, `AGENT`, `TYPE`, `MODEL`, `SKILL`, `MCP`, `VECTOR`, `GRAPH`,
  `MEMORY`, `LOOP`, `ENDPOINT`, `ENV`, `LABEL`, and `EXEC` directives.

## Multi-Agent Compose

- Line-oriented `AgentCompose` document with `COMPOSE` and `AGENT` directives.
- `agentctl compose ls`, `compose up`, `compose down`, `compose ps`.
- Topological start order from `DEPENDS_ON`; deterministic alphabetical
  tie-break.
- Compose-managed agents carry `agentctl.compose.project` and
  `agentctl.compose.service` labels for tear-down and filtering.

## Docker-Like UX

- `run`, `ps -a/-q/-aq`, `logs`, `trace` (human + `--json`), `inspect`,
  `describe`, `stop`, `start`, `restart`, `rm -f`.
- Singular and plural forms: `agent | agents`, `model | models`,
  `skill | skills`, `tool | tools`.

## Knowledge / Action / Persistence / Control

- `rag ls`, `rag vector ls`, `rag graph ls`.
- `tool ls`, `tool mcp ls`, `tool exec`, top-level `exec` alias.
- `memory ls`, `memory short ls`, `memory long ls`, `memory dump`,
  `memory recall`.
- `loop ls`, `loop ps`, `loop trace`.
- `guard ls` (placeholder; reserved for future GUARD directive).

## Health & Discovery

- `agentctl health <id>` probes `/health`, `/status`, and `/tasks` against the
  agent's first ENDPOINT (or `--url` override) and records a `health` trace
  event with ok/fail counts.
- `agentctl tool mcp ls <id>` discovers MCP tool schemas via
  `tools/list` JSON-RPC.
- `agentctl tool exec <id> <tool>` invokes `tools/call` and records a `tool`
  trace event with latency and status.

## Catalogs

- Role images: planner, researcher, coder, reviewer, executor, coordinator.
- Model providers: OpenAI, Anthropic, Gemini, vLLM, llama.cpp.

## CI and Release

- Formatting, vet, test, build.
- Sonar scan configuration.
- Syft SBOM workflow.
- Grype CVE scan workflow.
- Release workflow for archives, `.deb`, `.rpm`, checksums, and release SBOM.
