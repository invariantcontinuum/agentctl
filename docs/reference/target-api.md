# Target API

This page documents the desired end-state API. It is not a list of implemented commands.

```text
Usage:  agentctl [OPTIONS] COMMAND

Self-sufficient runtime for AI agents
Knowledge + Action + Persistence + Control + Specialization
```

## Common Commands

```text
run      Start resource and loop
ps/ls    List with status and metrics
logs     Reasoning, tool, RAG, and memory traces
exec     Run tool or step in context
trace    Full planning/execution history
inspect  Detailed machine-readable state
describe Human-readable state
rm       Remove stopped state, force-remove running resources
```

## Management Commands

```text
agent   Running agents and multi-agent teams
skill   Capabilities and playbooks
model   LLM backends
rag     Knowledge: vector, graph, hybrid retrieval
memory  Persistence: short, long, episodic, graph
tool    Action: MCP, tools, function calling
loop    Control: planning, orchestration, evaluation
guard   Guardrails, validation, safety
```

## Uniform Patterns

### Knowledge

```bash
agentctl rag ls
agentctl rag vector ls
agentctl rag graph ls
agentctl rag run "query"
agentctl rag trace <id>
```

### Action

```bash
agentctl tool ls
agentctl tool mcp ls
agentctl tool exec <agent> search "query"
agentctl tool trace <agent>
```

### Persistence

```bash
agentctl memory ls
agentctl memory short ls
agentctl memory long ls
agentctl memory dump <agent>
agentctl memory recall <agent> "key"
```

### Control

```bash
agentctl loop ls
agentctl loop ps <agent>
agentctl loop trace <agent>
agentctl guard ls <agent>
```

### Specialization

```bash
agentctl skill ls
agentctl agent run planner --skill decompose-tasks
agentctl agent ps
agentctl agent compose up
```

## Global Options

```text
--loop=react
--model=<provider-or-model>
--mcp=<server>
--debug
--compose
```

## Deferred

`network` commands are intentionally deferred until the network model is designed.
