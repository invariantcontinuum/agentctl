# Models

`agentctl models ls` lists provider definitions.

The model layer is intentionally abstract. The CLI should not contain OpenAI, Anthropic, Gemini, vLLM, or llama.cpp client logic directly. It should pass model configuration into the runtime and let a model driver or agent runtime resolve provider-specific behavior.

## Current Providers

```text
openai:default     hosted API, API key via OPENAI_API_KEY
anthropic:default  hosted API, API key via ANTHROPIC_API_KEY
gemini:default     hosted API, API key or OAuth-backed driver config
vllm:local         local OpenAI-compatible vLLM endpoint
llamacpp:local     local OpenAI-compatible llama.cpp endpoint
```

## Current Command

```bash
agentctl models ls
```

## Target Direction

Future commands should support Docker-like model management:

```bash
agentctl model ls
agentctl model inspect openai:default
agentctl model test vllm:local "hello"
agentctl model set-default llamacpp:local
```

Provider details should remain configurable through manifests, environment variables, and future provider config files.
