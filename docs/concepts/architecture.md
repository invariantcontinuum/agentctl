# Architecture

`agentctl` is organized around small package boundaries.

## Current Packages

- `cmd/agentctl`: CLI entrypoint.
- `internal/cli`: command parsing and presentation.
- `internal/agent`: domain model and validation.
- `internal/agentfile`: line-oriented manifest parser.
- `internal/catalog`: role image catalog.
- `internal/model`: model provider catalog.
- `internal/driver`: runtime driver interface and local process driver.
- `internal/store`: JSON-backed local state.

## Design Rules

- Keep command handlers thin.
- Put lifecycle behavior behind `driver.Driver`.
- Put role specialization behind the image catalog.
- Put model provider definitions behind the model catalog.
- Keep provider credentials out of manifests; reference environment variables instead.
- Keep `inspect` machine-readable and `describe` human-readable.

## Target Runtime

The long-term runtime should coordinate:

- Agent lifecycle.
- Planning loops.
- MCP tool calling.
- Vector/graph/hybrid RAG.
- Short and long-term memory.
- Guardrails and evaluation.
- Multi-agent delegation.

Each component should expose Docker-like operations: `run`, `ls`/`ps`, `logs`, `exec`, `trace`, and `inspect`.
