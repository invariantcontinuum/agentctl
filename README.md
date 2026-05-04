# agentctl

`agentctl` is a Docker-style control CLI for long-running AI agents. The first implementation slice is deliberately small: a Go standard-library CLI that parses an `Agentfile`, starts a local agent process, stores instance state, and exposes lifecycle commands that can later be backed by Docker, Podman, Kubernetes, or systemd drivers.

## Current Scope

- Standard-library Go project, no runtime dependencies.
- Line-oriented `Agentfile` parser with unit tests.
- Domain model for skills, MCP servers, vector RAG, graph RAG, memory, loop policy, endpoints, environment, labels, and process command.
- Local process driver for `run`, `start`, `stop`, and `restart`.
- JSON-backed state store under the user's config directory.
- CLI commands for `run`, `ps`, `logs`, `stop`, `start`, `restart`, `inspect`, `list-skills`, `list-tools`, and `trace`.

## Build

```bash
make build
```

## Test

```bash
make fmt
make lint
make test
```

## Try the Sample Agentfile

```bash
go run ./cmd/agentctl run -f Agentfile
go run ./cmd/agentctl ps
go run ./cmd/agentctl logs planner-local-<suffix>
go run ./cmd/agentctl inspect planner-local-<suffix>
go run ./cmd/agentctl stop planner-local-<suffix>
```

The sample uses `sh` and `sleep` to create a real long-running local process. It is a stand-in for an agent server binary that would expose `/health`, `/status`, `/tasks`, and an MCP endpoint.

## Agentfile

See [docs/agentfile.md](docs/agentfile.md) and the root [Agentfile](Agentfile).
