# agentctl

## Description

`agentctl` is a Docker-style control-plane CLI for long-running AI agents. It reads deterministic `Agentfile` manifests, starts and manages agent processes, stores local instance state, and exposes lifecycle commands that can grow into Docker, Podman, Kubernetes, or systemd-backed drivers.

The project is Go 1.26.2+, standard-library-first, and organized around explicit package boundaries for domain validation, `Agentfile` parsing, state persistence, runtime drivers, and CLI presentation.

The CLI intentionally follows Docker command shapes:

```bash
agentctl run --rm coder:latest
agentctl ps -aq
agentctl agent ls
agentctl model ls
agentctl describe coder-<suffix>
agentctl rm -f coder-<suffix>
```

Multi-agent teams compose like Docker services:

```bash
agentctl compose ls   -f examples/team/AgentCompose
agentctl compose up   -f examples/team/AgentCompose
agentctl compose ps   -f examples/team/AgentCompose
agentctl compose down -f examples/team/AgentCompose
```

Action and observability commands talk to the agent's MCP and HTTP surfaces:

```bash
agentctl tool mcp ls coder-<suffix>
agentctl exec --args '{"q":"agents"}' coder-<suffix> search
agentctl health planner-<suffix>
agentctl trace planner-<suffix>
agentctl logs --level warn planner-<suffix>
```

Authentication mirrors the Anthropic Agent SDK and OpenAI Agents SDK
patterns — API key per provider, persisted to a 0600 file:

```bash
agentctl model anthropic auth login                                  # interactive
agentctl model openai    auth login --api-key sk-... --no-interactive
agentctl model vllm      auth login --endpoint http://localhost:8000/v1
agentctl model auth ls
```

`Agentfile` supports Docker-like `FROM` inheritance and stdio MCP servers:

```text
FROM ../base/Agentfile
AGENT researcher-from-base
TYPE researcher
MCP search http  http://localhost:9001/mcp
MCP fs     stdio npx -y @modelcontextprotocol/server-filesystem /tmp
```

`network` commands are intentionally not implemented yet; that surface is planned separately.

## Documentation

The Material for MkDocs source lives in [docs](docs/index.md), with [mkdocs.yml](mkdocs.yml) defining the navigation. Start with:

- [Quickstart](docs/quickstart.md) for the current local runtime flow.
- [Implemented Commands](docs/reference/commands.md) for the CLI surface available today.
- [Target API](docs/reference/target-api.md) for the planned Docker-like agentic API.
- [What Is Next](docs/roadmap/todo.md) for the implementation roadmap.

## Install

Release assets are published for Windows, macOS, Linux tarballs, Debian/Ubuntu `.deb`, and RHEL/Fedora `.rpm` on x64 and ARM64.

Set the repository once for shell examples:

```bash
export AGENTCTL_REPO=invariantcontinuum/agentctl
```

Windows x64 PowerShell:

```powershell
$r=Invoke-RestMethod "https://api.github.com/repos/invariantcontinuum/agentctl/releases/latest"; $v=$r.tag_name.TrimStart("v"); New-Item -ItemType Directory -Force "$env:USERPROFILE\bin" | Out-Null; Invoke-WebRequest "https://github.com/invariantcontinuum/agentctl/releases/latest/download/agentctl_${v}_windows_amd64.zip" -OutFile "$env:TEMP\agentctl.zip"; Expand-Archive "$env:TEMP\agentctl.zip" -DestinationPath "$env:USERPROFILE\bin" -Force
```

Windows ARM64 PowerShell:

```powershell
$r=Invoke-RestMethod "https://api.github.com/repos/invariantcontinuum/agentctl/releases/latest"; $v=$r.tag_name.TrimStart("v"); New-Item -ItemType Directory -Force "$env:USERPROFILE\bin" | Out-Null; Invoke-WebRequest "https://github.com/invariantcontinuum/agentctl/releases/latest/download/agentctl_${v}_windows_arm64.zip" -OutFile "$env:TEMP\agentctl.zip"; Expand-Archive "$env:TEMP\agentctl.zip" -DestinationPath "$env:USERPROFILE\bin" -Force
```

macOS x64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_darwin_amd64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

macOS ARM64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_darwin_arm64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

Linux x64 tarball:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_amd64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

Linux ARM64 tarball:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_arm64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

Debian/Ubuntu x64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_amd64.deb" -o /tmp/agentctl.deb && sudo dpkg -i /tmp/agentctl.deb
```

Debian/Ubuntu ARM64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_arm64.deb" -o /tmp/agentctl.deb && sudo dpkg -i /tmp/agentctl.deb
```

RHEL/Fedora x64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_x86_64.rpm" -o /tmp/agentctl.rpm && sudo rpm -Uvh /tmp/agentctl.rpm
```

RHEL/Fedora ARM64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_aarch64.rpm" -o /tmp/agentctl.rpm && sudo rpm -Uvh /tmp/agentctl.rpm
```

From source:

```bash
git clone https://github.com/invariantcontinuum/agentctl.git && cd agentctl && make build && sudo install -m 0755 bin/agentctl /usr/local/bin/agentctl
```

## Usage

Parse and validate an `Agentfile` without starting it:

```bash
agentctl run -f Agentfile --dry-run
```

Start a role image with Docker-like `--rm` lifecycle behavior:

```bash
agentctl run --rm coder:latest
```

Start the sample local planner agent:

```bash
agentctl run -f Agentfile
```

List known agents:

```bash
agentctl ps
agentctl ps -aq
agentctl agents ls
```

Inspect an agent:

```bash
agentctl inspect planner-local-<suffix>
agentctl describe planner-local-<suffix>
agentctl agents describe planner-local-<suffix>
```

Read logs and trace:

```bash
agentctl logs planner-local-<suffix>
agentctl trace planner-local-<suffix>
```

Stop, start, or restart an agent:

```bash
agentctl stop planner-local-<suffix>
agentctl start planner-local-<suffix>
agentctl restart planner-local-<suffix>
```

Remove stopped agent state:

```bash
agentctl rm planner-local-<suffix>
agentctl rm -f planner-local-<suffix>
agentctl agents rm -f planner-local-<suffix>
```

List local skills and configured MCP tools (singular and plural both work):

```bash
agentctl skill ls ./skills
agentctl tool ls planner-local-<suffix>
agentctl tool mcp ls planner-local-<suffix>
```

List model provider definitions:

```bash
agentctl model ls
```

Probe runtime contract and structured trace:

```bash
agentctl health planner-local-<suffix>
agentctl trace planner-local-<suffix>
agentctl trace --json planner-local-<suffix>
```

When the Agentfile omits `EXEC`, the CLI runs the bundled `agentd` binary
(`./bin/agentd` after `make build`). It exposes `/health`, `/status`,
`/tasks`, `POST /tasks`, and `/tasks/{id}` — exactly what `agentctl health`
probes. Submit work with `curl`:

```bash
curl -X POST http://127.0.0.1:8088/tasks \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"summarise the last commit","system":"be brief"}'
```

Provider credentials saved with `agentctl model <provider> auth login`
are injected into the child process before `agentd` starts.

Inspect knowledge, persistence, and control bindings:

```bash
agentctl rag ls planner-local-<suffix>
agentctl memory ls planner-local-<suffix>
agentctl loop ls
agentctl loop ps planner-local-<suffix>
```

## Agentfile and AgentCompose

`Agentfile` describes one agent. See [docs/agentfile.md](docs/agentfile.md) and
the sample [Agentfile](Agentfile). Per-role examples live under
[examples/](examples/).

`AgentCompose` describes a team of agents. See
[docs/reference/agent-compose.md](docs/reference/agent-compose.md) and the
sample [examples/team/AgentCompose](examples/team/AgentCompose).

## Capability Taxonomy

`agentctl` keeps agent capabilities in a clean taxonomy:

- Knowledge: RAG, vector databases, graph databases, GraphRAG, and hybrid retrieval.
- Action: tools, MCP servers, function calling, APIs, code execution, search, databases, and file operations.
- Persistence: short-term context, conversation history, summaries, long-term memory, episodic memory, vector memory, and graph memory.
- Control: planning, reasoning, orchestration loops, evaluation, guardrails, tool permissioning, and completion criteria.
- Specialization: skills, `.md` playbooks, and role-specific agents such as planner, researcher, coder, reviewer, executor, and coordinator.

## Models

`agentctl models ls` lists model provider definitions, not hardcoded model clients. The provider abstraction covers:

- OpenAI-compatible hosted APIs using `OPENAI_API_KEY`.
- Anthropic hosted APIs using `ANTHROPIC_API_KEY`.
- Gemini hosted APIs using API keys or OAuth-backed driver configuration.
- Local OpenAI-compatible vLLM endpoints.
- Local OpenAI-compatible llama.cpp endpoints.

Agent images and `Agentfile` manifests bind to models through the `MODEL` directive. Credentials are referenced by environment variable name instead of being stored in the manifest.

## Development

```bash
make fmt
make fmt-check
make lint
make test
make build
```

## CI and Releases

CI runs formatting checks, `go vet`, tests, build, Syft SBOM generation, and Grype CVE scanning. Documentation deploys through the `Deploy Documentation` workflow, which builds the Squidfunk Material for MkDocs site with `mkdocs build --strict` and publishes the `site/` artifact to GitHub Pages.

Releases can be created by pushing a `vX.Y.Z` tag or by manually running the release workflow with `major`, `minor`, or `patch` bump input. Release assets include checksums, SBOM, archives, `.deb`, and `.rpm` packages.
