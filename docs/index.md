# agentctl

`agentctl` is a Docker-style control-plane CLI for AI agents.

The goal is a self-sufficient runtime for agents that makes the inner loop visible: planning steps, tool calls, RAG retrievals, memory reads/writes, guard checks, and multi-agent delegation should be inspectable through familiar command patterns.

## Current Focus

The current codebase is an early Go implementation focused on local process lifecycle and command grammar:

- `agentctl run` starts an agent from an `Agentfile` or role image.
- `agentctl ps` and `agentctl agents ls` list agent state.
- `agentctl logs`, `trace`, `inspect`, and `describe` expose local logs, traces, JSON state, and readable state.
- `agentctl rm` removes stopped agent state, with `-f` for running agents.
- `agentctl models ls` lists model provider definitions.

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
