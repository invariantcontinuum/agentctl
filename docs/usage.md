# Current CLI Usage

This page documents what is implemented today.

## Common Commands

```bash
agentctl run [-f Agentfile] [--dry-run] [--rm] [--name name] [--workdir dir] [image]
agentctl ps [-a] [-q] [-aq]
agentctl logs <agent-id>
agentctl trace <agent-id>
agentctl inspect <agent-id>
agentctl describe <agent-id>
agentctl stop <agent-id>
agentctl start <agent-id>
agentctl restart <agent-id>
agentctl rm [-f|--force] <agent-id>...
```

## Grouped Commands

```bash
agentctl agents ls [-a] [-q] [-aq]
agentctl agents describe <agent-id>
agentctl agents rm [-f|--force] <agent-id>...
agentctl models ls
agentctl skills ls [directory...]
agentctl tools ls <agent-id>
```

## Aliases Retained for Compatibility During Early Development

```bash
agentctl list-skills [directory...]
agentctl list-tools <agent-id>
```

The long-term API should prefer noun groups: `skill`, `model`, `rag`, `memory`, `tool`, `loop`, `guard`, and `agent`.

## Not Implemented Yet

These are target commands, not current commands:

```bash
agentctl exec ...
agentctl stats
agentctl compose up
agentctl rag ...
agentctl memory ...
agentctl loop ...
agentctl guard ...
```

`network` commands are intentionally omitted until the network model is designed.
