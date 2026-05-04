# Agent SDK (`internal/agentsdk`)

`agentctl` ships a provider-neutral Go agent runtime that maps the
patterns from the
[Anthropic Agent SDK](https://code.claude.com/docs/en/agent-sdk/overview),
the [OpenAI Agents SDK](https://developers.openai.com/api/docs/guides/agents),
and [Google ADK-Go](https://github.com/google/adk-go) onto a single
SOLID/DRY interface set. Every layer is exposed so user code can compose
the same building blocks the bundled `agentd` runtime uses.

## Runnable

`Runnable` is the smallest "run on input, return result" surface. Every
agent type implements it, including the orchestrators below. This is the
glue that lets a `SequentialAgent` host a `LoopAgent` whose child is itself
a `HandoffAgent`.

```go
type Runnable interface {
    Name() string
    Run(ctx context.Context, session Session, input string) (RunResult, error)
}
```

## Agent

`Agent` is one model + tool loop. It mirrors the shape that all three SDKs
agree on: a model, a system prompt, a list of tools, optional hooks, and
optional guardrails.

```go
agent := agentsdk.NewAgent("planner", agentsdk.NewAnthropicClient(endpoint, key, model, nil))
agent.System = "You are a careful planner."
agent.Tools.Register(searchTool)
agent.Guards = []agentsdk.Guardrail{&agentsdk.MaxLengthGuard{GuardName: "len", Max: 8000}}
agent.Hooks.AfterTool = func(ctx context.Context, tool agentsdk.Tool, output string, err error) {
    log.Printf("tool %s -> %d bytes (err=%v)", tool.Name(), len(output), err)
}
result, err := agent.Run(ctx, session, "draft a plan")
```

The loop:

1. Append the user input to the session.
2. Call the model with `(system, messages, tools)`.
3. Append the assistant turn.
4. If the assistant emitted `tool_use` blocks, dispatch each through the
   tool registry and append a single `tool_result` user message back.
5. Repeat until `end_turn` or `MaxSteps`.

## Multi-agent orchestrators

```go
sequence  := &agentsdk.SequentialAgent{Children: []agentsdk.Runnable{planner, executor}}
fanout    := &agentsdk.ParallelAgent{Children: []agentsdk.Runnable{searcher, summarizer}}
loop      := &agentsdk.LoopAgent{Child: critic, MaxIterations: 4, Predicate: hasNoIssues}
handoff   := &agentsdk.HandoffAgent{Router: triager, Children: map[string]agentsdk.Runnable{
    "billing": billing, "support": support,
}}
```

`Sequential`, `Parallel`, and `Loop` mirror Google ADK-Go's orchestrators of
the same name. `Handoff` mirrors the OpenAI Agents SDK handoff primitive,
where one agent's output names the next agent to run.

## Tools

`Tool` is the open extension point.

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, input json.RawMessage) (string, error)
}
```

Two concrete implementations ship:

- `FunctionTool` wraps a Go function; great for in-process utilities.
- `MCPTool` wraps one tool exposed by an MCP server. Discovery via
  `agentsdk.DiscoverMCPTools(ctx, mcpClient, mcpServers, onError)` returns
  one `MCPTool` per remote tool.

## Sessions

`Session` is the conversation persistence surface.

- `MemorySession` keeps history in process — right for one-shot tasks.
- `FileSession` persists JSON-Lines so a future `agentctl session resume`
  can replay the transcript.

## Hooks and guardrails

`Hooks` covers `BeforeRun`, `AfterRun`, `BeforeTool`, `AfterTool` — the
agent fires whichever fields are non-nil. `Guardrail` is the OpenAI-style
content policy check that runs against every assistant text emission.

## Provider matrix

| Provider          | Client                        | Tool use        | Notes                                      |
| ----------------- | ----------------------------- | --------------- | ------------------------------------------ |
| Anthropic         | `NewAnthropicClient`          | `tool_use`      | Messages API, x-api-key + anthropic-version|
| OpenAI            | `NewOpenAIClient`             | `function tools`| Chat Completions; works for vLLM, llama.cpp|
| Google Gemini     | `NewGeminiClient`             | `functionCall`  | generateContent + functionDeclarations     |
| Echo (fallback)   | `NewEchoClient`               | _ignored_       | Deterministic; used when no key is set     |

The agentctl CLI auto-selects the right client at `run` time based on the
`MODEL provider` directive and the credentials store; manual construction
is only needed for embedding scenarios.
