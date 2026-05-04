# AgentCompose

`AgentCompose` is the multi-agent companion to `Agentfile`. Each `AGENT` line
references one `Agentfile` by relative path, and `DEPENDS_ON` declares a
topological start order so `agentctl compose up` brings dependencies online
first.

The format is line-oriented so the standard library is enough to parse it.
Blank lines and `#` comments are ignored.

## Grammar

```text
COMPOSE <project-name>
AGENT <name> FILE=<agentfile-path> [DEPENDS_ON=<csv>]
```

- `COMPOSE` declares the project name. Compose-managed agents are tagged with
  `agentctl.compose.project=<name>` and `agentctl.compose.service=<service>`.
  These labels are how `compose ps` and `compose down` find their work.
- `AGENT <name>` registers a service. `<name>` must be unique within the
  document.
- `FILE=` is required. Relative paths resolve against the directory holding
  the AgentCompose file.
- `DEPENDS_ON=` is optional and accepts a comma-separated list of service
  names that must be started first. Each dependency must reference a service
  declared in the same document.

## Example

```text
# examples/team/AgentCompose
COMPOSE delivery-team

AGENT planner     FILE=../planner/Agentfile
AGENT researcher  FILE=../researcher/Agentfile  DEPENDS_ON=planner
AGENT coder       FILE=../coder/Agentfile       DEPENDS_ON=planner,researcher
AGENT executor    FILE=../executor/Agentfile    DEPENDS_ON=coder
AGENT reviewer    FILE=../reviewer/Agentfile    DEPENDS_ON=coder
AGENT coordinator FILE=../coordinator/Agentfile DEPENDS_ON=planner,researcher,coder,reviewer,executor
```

## Commands

```bash
agentctl compose ls   -f examples/team/AgentCompose
agentctl compose up   -f examples/team/AgentCompose
agentctl compose ps   -f examples/team/AgentCompose
agentctl compose down -f examples/team/AgentCompose
```

`compose up --dry-run` validates the document and prints the resolved start
order without launching any agent.

## Topological Sort

`Plan()` uses Kahn's algorithm. Equal-rank nodes are sorted alphabetically so
the order is deterministic and stable across runs. A cyclic `DEPENDS_ON`
graph fails fast with `cyclic depends_on graph` and no agents are started.

## Trace Events

Every compose-started agent records a `run` trace event with the additional
fields `compose=<project>` and `service=<service>`. `compose down` emits a
matching `stop` event before removing the recorded state.
