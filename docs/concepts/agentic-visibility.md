# Agentic Visibility

Docker shows container status, logs, and inspectable configuration. `agentctl` should show the same operational surface plus agent-loop internals.

## Target Visibility

Every operational command should answer agent-specific questions:

- What step is the loop on?
- What was the current or last plan?
- Which RAG sources were queried?
- Which memory entries were recalled or written?
- Which MCP tools were called?
- Which guard checks ran?
- What is the next action?

## Target `inspect`

```text
agentctl agent inspect my-planner

Skills: decompose-tasks, reflect
RAG: vector=docs(85% hit), graph=tasks
Memory: short=2.3k tokens, long=47 episodes
Loop: ReAct(step=7/20), Tools: 3 calls
Guard: tool-permission=strict, eval=pass
```

## Target `trace`

```text
agentctl agent trace my-planner --since 10m

Plan: decompose(user_query)
RAG: vector="docs/install.md" score=0.92
Tool: mcp-search("docker agentctl")
Reflect: search insufficient, need graph
RAG: graph="tasks:docker->agentctl"
Action: delegate->coder-agent
```

## Current State

The current implementation records local lifecycle trace events such as `run`, `start`, `stop`, and forced `rm`.

Planning/RAG/tool/memory/guard events are design targets.
