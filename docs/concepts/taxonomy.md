# Capability Taxonomy

`agentctl` uses a five-part taxonomy for agent capabilities.

## Knowledge

Knowledge is retrieval and context:

- Vector RAG.
- GraphRAG.
- Hybrid retrieval.
- Vector databases.
- Graph databases.
- Document indexes and retrieval metrics.

Target command family:

```bash
agentctl rag ls
agentctl rag vector ls
agentctl rag graph ls
agentctl rag run "query"
agentctl rag trace <id>
```

## Action

Action is how agents affect the outside world:

- MCP servers.
- Function calling.
- APIs.
- Code execution.
- Search.
- Databases.
- File operations.

Target command family:

```bash
agentctl tool ls
agentctl tool mcp ls
agentctl tool exec <agent> search "query"
agentctl tool trace <agent>
```

## Persistence

Persistence is memory:

- Short-term context.
- Conversation history.
- Summaries.
- Long-term memory.
- Episodic memory.
- Vector memory.
- Graph memory.

Target command family:

```bash
agentctl memory ls
agentctl memory short ls
agentctl memory long ls
agentctl memory dump <agent>
agentctl memory recall <agent> "key"
```

## Control

Control is planning, orchestration, evaluation, and guardrails:

- Goal decomposition.
- Step sequencing.
- Branching.
- Reflection.
- Next-action selection.
- Tool permissioning.
- Completion criteria.

Target command family:

```bash
agentctl loop ls
agentctl loop ps <agent>
agentctl loop trace <agent>
agentctl guard ls <agent>
```

## Specialization

Specialization is role and behavior:

- Skills and playbooks.
- Planner, researcher, coder, reviewer, executor, coordinator roles.
- Multi-agent delegation.
- Team composition.

Target command family:

```bash
agentctl skill ls
agentctl agent run planner --skill decompose-tasks
agentctl agent ps
agentctl agent compose up
```
