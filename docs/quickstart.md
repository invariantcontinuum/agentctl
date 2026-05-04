# Quickstart

## Build the CLI and Runtime

```bash
make build
```

This produces two binaries: `bin/agentctl` (the CLI) and `bin/agentd`
(the bundled runtime that hosts a single agent).

## Validate the Sample Agentfile

```bash
./bin/agentctl run -f Agentfile --dry-run
```

This prints the parsed agent config as JSON without spawning a process.

## Start an Agent from a Role Image

```bash
./bin/agentctl run --rm coder:latest
```

`--rm` records Docker-like cleanup behavior: when the agent is stopped,
its local state is removed.

When the Agentfile omits `EXEC`, `agentctl run` injects
`agentd --config <path> --addr <host:port>` as the runtime, so a
minimal Agentfile is enough to boot a working agent with the standard
runtime contract (`/health`, `/status`, `/tasks`, `POST /tasks`,
`/tasks/{id}`).

## Submit a Task

Once the agent is running, post work to it directly:

```bash
curl -X POST http://127.0.0.1:8088/tasks \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"summarise the last commit","system":"be brief"}'
```

The response is the queued `Task`; poll `GET /tasks/{id}` for the
result. Provider credentials saved with `agentctl model <provider>
auth login` are injected into the agent before launch.

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
