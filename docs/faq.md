# FAQ

## Is `agentctl` production-ready?

No. The project is pre-MVP. The current implementation is a local-process
control plane with a bundled `agentd` runtime, but the richer network, session,
guard, live RAG, and live memory surfaces are still roadmap items.

## Why use Docker-like commands?

Docker gives operators a predictable vocabulary: `run`, `ps`, `logs`, `exec`, `inspect`, `rm`. `agentctl` keeps that uniformity while adding visibility into planning loops, RAG, memory, tools, and guardrails.

## Why is `network` not implemented?

The network surface is deliberately deferred. Agent networking, MCP server reachability, and multi-agent team communication need a separate design before commands are added.

## Is `models ls` listing real remote models?

Not yet. It lists provider definitions and local endpoint bindings. Provider-specific discovery is future work.

## Where do secrets go?

Secrets go in the credentials store via `agentctl model <provider> auth login`
or in the process environment. `Agentfile` references env names such as
`OPENAI_API_KEY` through `api_key_env`; it should not contain raw secret
values.

## What is the difference between `inspect` and `describe`?

`inspect` is JSON for automation. `describe` is human-readable output for operators.

## What is the difference between `trace` and `logs`?

`logs` prints process output. `trace` prints structured lifecycle, tool, and
health events today; deeper planning, RAG, memory, guard, and delegation events
are target runtime surfaces.

## Does `--rm` behave like Docker?

It follows the same intent. If an agent is started with `--rm`, stopping it removes recorded local state and log/trace files.

## What is the target command family?

The implemented families are `agent`, `skill`, `model`, `rag`, `memory`,
`tool`, `loop`, and `guard`. Some commands in those families are placeholders
or metadata-only until the runtime contract grows.
