# Agentfile

`Agentfile` is a small line-oriented manifest for an agent service. It is
intentionally not YAML so `agentctl` can parse it with the Go standard library
and keep the grammar explicit.

Blank lines and `#` comments are ignored.

Every non-`EXEC` directive is parsed with whitespace-separated fields. Put
structured data in `key=value` options and avoid spaces inside values. `EXEC`
is the one exception: it is parsed as a JSON string array so command arguments
remain deterministic.

## Directives

```text
FROM <parent-agentfile-path>
AGENT <name>
IMAGE <agent-image-ref>
TYPE <agent-type>
MODEL <provider> <name> [base_url=<url>] [auth=<api_key|oauth|api_key_or_oauth|none>] [api_key_env=<env-var>] [timeout_sec=<seconds>] [option.<key>=<value>]
SKILL <path-or-registry-name> [id=<id>] [name=<name>] [type=<prompt|markdown|function|plugin|builtin>] [path=<path-or-url>] [content=<inline-text>] [depends_on=<id,id>] [enabled=<bool>] [metadata.<key>=<value>]
MCP <name> http  <url> [base_path=<path>] [timeout_sec=<seconds>] [header.<key>=<value>] [enabled=<bool>]
MCP <name> stdio <command> [arg ...] [url=<url>] [base_path=<path>] [timeout_sec=<seconds>] [env.<key>=<value>] [enabled=<bool>]
MCP_TOOL <server> <id> <name> [description=<text>] [category=<category>] [enabled=<bool>] [metadata.<key>=<value>]
VECTOR <name> <provider> <url> [index] [embedding_model=<model>] [weight=<number>] [label.<key>=<value>] [metadata.<key>=<value>]
GRAPH <name> <provider> <url> [index] [label.<key>=<value>] [metadata.<key>=<value>]
MEMORY <name> <type> <provider> [url-or-bucket] [url=<url>] [bucket=<bucket>] [limit=<entries-or-tokens>] [ttl_sec=<seconds>] [label.<key>=<value>] [metadata.<key>=<value>]
LOOP <name> max_steps=<positive-int> [max_tokens=<tokens>] [tool_selection=<mcp|direct|auto>]
HOOK <pre|post> <name> <type> [url=<url>] [timeout_sec=<seconds>] [on_error=<continue|halt|retry>] [header.<key>=<value>] [label.<key>=<value>]
EVALUATION [max_errors=<count>] [tool_allow_list=<tool,tool>] [tool_deny_list=<tool,tool>] [log_filter=<term,term>] [completion_criteria=<name,name>]
VALIDATOR_TOOL <id> <name> [description=<text>] [category=<category>] [enabled=<bool>] [metadata.<key>=<value>]
MULTI_AGENT [enabled=<bool>] [coordinator=<role>] [allowed_roles=<role,role>] [delegation=<always|policy|never>] [policy.<key>=<value>]
ENDPOINT <name> <url>
ENDPOINT <name> scheme=<scheme> host=<host> [port=<port>] [path=<path>] [label.<key>=<value>]
ENV <key>=<value>
LABEL <key>=<value>
EXEC ["program", "arg1", "arg2"]
```

The parsed result is the canonical `agent.Config` JSON shape used by
`agentctl run`, persisted runtime config files, and `agentd`.

## FROM (Docker-like inheritance)

Agentfiles compose the same way Dockerfiles do. A child Agentfile that begins
with

```text
FROM ./base/Agentfile
```

inherits every directive from the parent before its own directives are
applied. Merge semantics:

- Single-value directives (`AGENT`, `IMAGE`, `TYPE`, `MODEL`, `LOOP`, `EXEC`)
  in the child override the parent.
- List directives (`SKILL`, `MCP`, `MCP_TOOL`, `VECTOR`, `GRAPH`, `MEMORY`,
  `HOOK`, `VALIDATOR_TOOL`, and `ENDPOINT`) in the child append to the
  parent's list.
- Partial loop directives (`EVALUATION`, `MULTI_AGENT`) merge into the current
  loop configuration.
- Map directives (`ENV`, `LABEL`) in the child merge with the parent. Child
  wins on key conflict.

`FROM` is resolved relative to the file that contains it. Cycles are detected
and rejected. `FROM` is only valid via on-disk parsing — streaming `Parse(r)`
calls reject it because there's no path to resolve against.

See `examples/base/Agentfile` and `examples/from-base/Agentfile` for a working
pair.

## MCP transports

`MCP` accepts either an HTTP endpoint or a stdio child process, matching the
Anthropic Agent SDK and OpenAI Agents SDK MCP shapes.

```text
MCP search http  http://localhost:9001/mcp
MCP fs     stdio npx -y @modelcontextprotocol/server-filesystem /tmp
```

The transport keyword (`http` or `stdio`) is required by the Agentfile grammar
but is not stored as a separate domain field. HTTP directives fill `URL`;
stdio directives fill `Command` and `Args`. `agentctl tool mcp ls` discovers
tools over both transports; `agentctl tool exec` invokes `tools/call` over the
matching transport.

`MCP_TOOL` records an allow-listed tool descriptor under an MCP server. Runtime
tool discovery still comes from `tools/list`; the manifest tool list is the
static policy surface for future validation and presentation.

HTTP MCP servers can carry static request headers:

```text
MCP search http http://localhost:9001 header.Authorization=Bearer_test
```

Stdio MCP servers can carry child-process environment values:

```text
MCP fs stdio npx -y @modelcontextprotocol/server-filesystem /tmp env.ROOT=/tmp
```

## MODEL bindings

`MODEL` is a provider binding, not provider-specific client logic. Hosted
providers such as OpenAI, Anthropic, and Gemini should use `auth=api_key` or
`auth=api_key_or_oauth` with credentials referenced by environment variable
name. Local providers such as vLLM and llama.cpp should use an
OpenAI-compatible local endpoint with `auth=none`.

API keys can also be persisted with `agentctl model <provider> auth login`;
see [Models and Auth](models.md).

```text
MODEL openai default base_url=https://api.openai.com/v1 auth=api_key api_key_env=OPENAI_API_KEY
MODEL anthropic default base_url=https://api.anthropic.com auth=api_key api_key_env=ANTHROPIC_API_KEY
MODEL gemini default base_url=https://generativelanguage.googleapis.com auth=api_key_or_oauth api_key_env=GEMINI_API_KEY
MODEL vllm local base_url=http://localhost:8000/v1 auth=none
MODEL llamacpp local base_url=http://localhost:8102/v1 auth=none
```

## Skills, RAG, Memory, and Loop Policy

These directives map directly to the canonical `agent.Config` children:
`Skill`, `RAGSource`, `Memory`, `Loop`, `Hook`, `Evaluation`, and
`MultiAgentConfig`.

```text
SKILL ./skills/planner.md name=planner type=markdown
VECTOR docs pgvector postgres://localhost:5432/agentctl docs_chunks embedding_model=bge-m3
GRAPH tasks neo4j bolt://localhost:7687 task_graph
MEMORY session short inmemory limit=12000 ttl_sec=3600
HOOK pre audit http url=http://localhost:9010/pre on_error=halt timeout_sec=5
EVALUATION max_errors=3 tool_allow_list=search.web,fs.read completion_criteria=task_done,timeout
MULTI_AGENT enabled=true coordinator=coordinator allowed_roles=coder,reviewer delegation=policy policy.max_parallel=2
```

`SKILL content=<text>` inlines a prompt fragment. If `content` is present and
`path` is not explicitly set, the parser clears the inherited positional path
so `agentd` uses the inline content directly.

## Runtime contract

The process started by `EXEC` exposes:

- `GET /health` — liveness, returns `{status,agent,started}`
- `GET /status` — provider, role, queued/running/done/error counts
- `GET /tasks` — list of every task in submission order
- `POST /tasks` — submit `{prompt,system}`; replies `202` with the queued Task
- `GET /tasks/{task_id}` — read one task

The agent process also writes JSON-Lines log records of the shape

```json
{"ts":"2026-05-04T10:00:00Z","level":"info","msg":"started","fields":{"task":"123"}}
```

so `agentctl logs --level <level> <id>` can filter them. Lines that aren't
valid JSON are still printed as info-level lines, so non-conforming agents
remain readable.

### Default `agentd` runtime

When an `Agentfile` does not provide an `EXEC` directive, the CLI injects
the bundled `agentd` binary as the runtime:

```text
agentd --config <state-dir>/configs/<id>.json --addr <host:port>
```

`<host:port>` is taken from `ENDPOINT http <url>`; if no HTTP endpoint is
declared, `127.0.0.1:8088` is used. `agentd` builds an
[Agent SDK](concepts/agent-sdk.md) `Agent` from the Agentfile: the
`MODEL provider` selects the client (Anthropic Messages, OpenAI Chat
Completions, Gemini generateContent, or the deterministic Echo fallback);
each `SKILL` file becomes part of the system prompt; each `MCP` server is
discovered via `tools/list` and exposed as callable tools through the
agent loop. The runtime then exposes the HTTP contract above. Custom
`EXEC` programs are still supported and must implement the same HTTP
surface to satisfy `agentctl health`.

### Credential injection at run time

Before `agentd` starts, `agentctl run` and `agentctl compose up` look up
`MODEL provider` in the credentials store
(`${XDG_CONFIG_HOME}/agentctl/credentials.json`). If a record exists:

- the `api_key` is exported under the configured `api_key_env` (or the
  catalog default — e.g. `ANTHROPIC_API_KEY`);
- the stored `endpoint` overrides the Agentfile `base_url`;
- any `extra_env` keys (e.g. `CLAUDE_CODE_USE_BEDROCK=1`) are merged into
  the agent process environment.

`compose up` then probes `/health` for each service before starting the
next one in the topological order, so `DEPENDS_ON` boundaries match
actual readiness.
