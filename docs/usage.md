# Current CLI Usage

## Lifecycle

```bash
agentctl run [-f Agentfile] [--dry-run] [--rm] [--name name] [--workdir dir] [image]
agentctl ps [-a] [-q] [-aq]
agentctl logs [--level debug|info|warn|error] [--json] <agent-id>
agentctl trace [--json] <agent-id>
agentctl inspect <agent-id>
agentctl describe <agent-id>
agentctl stop <agent-id>
agentctl start <agent-id>
agentctl restart <agent-id>
agentctl rm [-f|--force] <agent-id>...
```

## Compose

```bash
agentctl compose ls   [-f path]
agentctl compose up   [-f path] [--dry-run]
agentctl compose down [-f path]
agentctl compose ps   [-f path]
```

## Action

```bash
agentctl tool ls <agent-id>
agentctl tool mcp ls <agent-id>
agentctl tool exec [--server NAME] [--args JSON] <agent-id> <tool>
agentctl exec [--server NAME] [--args JSON] <agent-id> <tool>
```

## Models and Auth

```bash
agentctl model ls
agentctl model anthropic auth login                              # interactive
agentctl model openai    auth login --api-key sk-... --no-interactive
agentctl model vllm      auth login --endpoint http://localhost:8000/v1
agentctl model anthropic auth status
agentctl model anthropic auth logout
agentctl model auth ls
```

## Knowledge / Persistence / Control

```bash
agentctl rag    ls <agent-id>
agentctl rag    vector ls <agent-id>
agentctl rag    graph  ls <agent-id>

agentctl memory ls <agent-id>
agentctl memory short ls <agent-id>
agentctl memory long  ls <agent-id>
agentctl memory dump  <agent-id>
agentctl memory recall <agent-id> <key>

agentctl loop  ls
agentctl loop  ps     <agent-id>
agentctl loop  trace  <agent-id>
agentctl guard ls     <agent-id>
```

## Health

```bash
agentctl health [--url URL] [--json] <agent-id>
```

## Management Aliases

Singular and plural noun groups both work:

```bash
agentctl agent ls   |  agentctl agents ls
agentctl model ls   |  agentctl models ls
agentctl skill ls   |  agentctl skills ls
agentctl tool ls    |  agentctl tools ls
```

## Compatibility Shims

```bash
agentctl list-skills [directory...]
agentctl list-tools <agent-id>
```

## Not Implemented Yet

These are target commands, not current commands. See `docs/roadmap/todo.md`
for the planned next steps:

```bash
agentctl rag run "query"
agentctl rag trace <id>
agentctl tool trace <agent>
agentctl stats
agentctl session ls / fork / resume     # mapped from Anthropic/OpenAI agent SDKs
```

`network` commands are intentionally omitted until the network model is
designed.
