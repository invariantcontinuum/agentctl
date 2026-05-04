# agentctl

`agentctl` is a Docker-style control-plane CLI for AI agents.

The goal is a self-sufficient runtime for agents that makes the inner loop visible: planning steps, tool calls, RAG retrievals, memory reads/writes, guard checks, and multi-agent delegation should be inspectable through familiar command patterns.

## Current Focus

The codebase is a Go implementation that already runs agents end-to-end
on the local machine. It ships two binaries:

- `agentctl` — the Docker-style CLI.
- `agentd` — the bundled runtime that hosts a single agent and serves
  the runtime contract (`/health`, `/status`, `/tasks`, `POST /tasks`,
  `/tasks/{id}`). `agentctl run` defaults `EXEC` to `agentd` whenever
  the Agentfile omits its own command, so a one-line Agentfile is
  enough to boot a working agent.

What works today:

- `agentctl run` / `compose up` from an `Agentfile` (with Docker-like
  `FROM` inheritance) or `AgentCompose` (topological order with
  `/health` gating between services).
- `agentctl ps`, `agent[s] ls / rm / describe`, `agentctl rm -f`,
  `stop`, `start`, `restart`.
- `agentctl logs --level <L> [--json]`, `trace [--json]`, `inspect`,
  `describe`.
- `agentctl model[s] ls`, plus per-provider auth:
  `model <provider> auth login | logout | status` and
  `model auth ls` against the credential store.
- `agentctl exec`, `tool ls / mcp ls / exec`, `health`.
- `agentctl rag ls`, `memory ls`, `loop ls`, `guard ls`,
  `skill[s] ls`.
- An `internal/agentsdk` package mirroring the Anthropic Agent SDK,
  OpenAI Agents SDK, and Google ADK-Go shapes: `Agent`, `Tool`,
  `Session`, `Hooks`, `Guardrail`, plus `Sequential` / `Parallel` /
  `Loop` / `Handoff` / `Isolated` orchestrators behind one `Runnable`
  interface.

## Long-Term Direction

The final API should feel like Docker while exposing agentic runtime internals:

```text
Usage:  agentctl [OPTIONS] COMMAND

Self-sufficient runtime for AI agents
Knowledge + Action + Persistence + Control + Specialization
```

The target command families are:

- `agent`: running agents and multi-agent teams.
- `skill`: capabilities and playbooks.
- `model`: hosted and local LLM backends.
- `rag`: vector, graph, and hybrid retrieval.
- `memory`: short-term, long-term, episodic, vector, and graph memory.
- `tool`: MCP servers, function calling, and external actions.
- `loop`: planning, execution, reflection, branching, and evaluation.
- `guard`: tool permissions, validation, safety, and completion criteria.

## Core Principle

Docker uniformity plus agentic transparency:

```bash
agentctl run --rm coder:latest
agentctl ps -aq
agentctl logs coder-<suffix>
agentctl trace coder-<suffix>
agentctl inspect coder-<suffix>
agentctl describe coder-<suffix>
```

Every `ps`, `logs`, `trace`, `inspect`, and `describe` surface should eventually expose loop internals: current step, RAG sources used, tools called, memory state, guard decisions, and next action.
