# Quickstart

## Build the CLI

```bash
make build
```

## Validate the Sample Agentfile

```bash
./bin/agentctl run -f Agentfile --dry-run
```

This prints the parsed agent config as JSON.

## Start an Agent from a Role Image

```bash
./bin/agentctl run --rm coder:latest
```

The current local driver starts a long-running local process. The `--rm` flag records Docker-like cleanup behavior: when the agent is stopped, its local state is removed.

## List Agents

```bash
./bin/agentctl ps
./bin/agentctl ps -aq
./bin/agentctl agents ls
```

## Inspect and Describe

`inspect` is JSON for automation:

```bash
./bin/agentctl inspect coder-<suffix>
```

`describe` is human-readable:

```bash
./bin/agentctl describe coder-<suffix>
./bin/agentctl agents describe coder-<suffix>
```

## Logs and Trace

```bash
./bin/agentctl logs coder-<suffix>
./bin/agentctl trace coder-<suffix>
```

The current trace file records lifecycle events. The target runtime will include planning, RAG, tool, memory, reflection, guard, and delegation events.

## Stop or Remove

```bash
./bin/agentctl stop coder-<suffix>
./bin/agentctl rm coder-<suffix>
./bin/agentctl rm -f coder-<suffix>
```

`rm` refuses running agents unless `-f` or `--force` is supplied.
