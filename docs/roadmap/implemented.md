# What Exists Now

## Runtime

- Local process driver.
- JSON-backed local state.
- XDG config/cache state locations.
- Lifecycle trace file for local runtime events.

## Agent Manifests

- Line-oriented `Agentfile`.
- `IMAGE`, `AGENT`, `TYPE`, `MODEL`, `SKILL`, `MCP`, `VECTOR`, `GRAPH`, `MEMORY`, `LOOP`, `ENDPOINT`, `ENV`, `LABEL`, and `EXEC` directives.

## Docker-Like UX

- `run`
- `ps -a/-q/-aq`
- `agents ls`
- `logs`
- `trace`
- `inspect`
- `describe`
- `stop`
- `start`
- `restart`
- `rm -f`

## Catalogs

- Role images: planner, researcher, coder, reviewer, executor, coordinator.
- Model providers: OpenAI, Anthropic, Gemini, vLLM, llama.cpp.

## CI and Release

- Formatting, vet, test, build.
- Sonar scan configuration.
- Syft SBOM workflow.
- Grype CVE scan workflow.
- Release workflow for archives, `.deb`, `.rpm`, checksums, and release SBOM.
