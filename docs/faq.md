# FAQ

## Is `agentctl` production-ready?

No. The project is pre-MVP. The current implementation is a local-process control-plane skeleton and documentation foundation.

## Why use Docker-like commands?

Docker gives operators a predictable vocabulary: `run`, `ps`, `logs`, `exec`, `inspect`, `rm`. `agentctl` keeps that uniformity while adding visibility into planning loops, RAG, memory, tools, and guardrails.

## Why is `network` not implemented?

The network surface is deliberately deferred. Agent networking, MCP server reachability, and multi-agent team communication need a separate design before commands are added.

## Is `models ls` listing real remote models?

Not yet. It lists provider definitions and local endpoint bindings. Provider-specific discovery is future work.

## Where do secrets go?

Secrets should be environment variables or future credential store entries. `Agentfile` should reference names such as `OPENAI_API_KEY`, not contain secret values.

## What is the difference between `inspect` and `describe`?

`inspect` is JSON for automation. `describe` is human-readable output for operators.

## What is the difference between `trace` and `logs`?

`logs` prints process output. `trace` prints structured lifecycle events today and should eventually print planning, RAG, tool, memory, guard, and delegation events.

## Does `--rm` behave like Docker?

It follows the same intent. If an agent is started with `--rm`, stopping it removes recorded local state and log/trace files.

## What is the target command family?

The target families are `agent`, `skill`, `model`, `rag`, `memory`, `tool`, `loop`, and `guard`.
