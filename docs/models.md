# Models and Auth

`agentctl` keeps model providers as configuration, not provider-specific
client logic. `agentctl model ls` lists the catalog and marks providers that
have credentials persisted on disk.

## Catalog

| Provider    | Kind   | Auth              | Endpoint                                        |
| ----------- | ------ | ----------------- | ----------------------------------------------- |
| `openai`    | hosted | api_key           | https://api.openai.com/v1                       |
| `anthropic` | hosted | api_key           | https://api.anthropic.com                       |
| `gemini`    | hosted | api_key or oauth  | https://generativelanguage.googleapis.com       |
| `vllm`      | local  | none              | http://localhost:8000/v1                        |
| `llamacpp`  | local  | none              | http://localhost:8102/v1                        |

## Authentication

Both the [Anthropic Agent SDK](https://code.claude.com/docs/en/agent-sdk/overview)
and the OpenAI Agents SDK authenticate by API key. `agentctl model <provider> auth ...`
stores those keys (and any provider-specific extra env vars like
`CLAUDE_CODE_USE_BEDROCK`) in
`${XDG_CONFIG_HOME}/agentctl/credentials.json` with mode `0600`.

```bash
agentctl model anthropic auth login                                  # interactive: prompts for endpoint + API key
agentctl model anthropic auth login --api-key sk-ant-... --no-interactive
agentctl model openai    auth login --api-key sk-... --endpoint https://api.openai.com/v1
agentctl model vllm      auth login --endpoint http://localhost:8000/v1   # local: no key needed
agentctl model anthropic auth status                                 # prints endpoint + masked key
agentctl model anthropic auth logout                                 # forgets credentials
agentctl model auth ls                                               # lists every logged-in provider
```

The CLI prefers POSIX `stty -echo` to mute interactive secret prompts. When
`stty` isn't available (Windows, restricted shells) the prompt falls back to
visible echo and prints a notice — pass `--api-key` or pipe stdin instead.

API keys are not mirrored back into `Agentfile` manifests. At `run` and
`compose up` time the CLI auto-injects the stored key under
`api_key_env` (defaulting to the catalog name — `OPENAI_API_KEY`,
`ANTHROPIC_API_KEY`, `GEMINI_API_KEY`), overrides `MODEL base_url` from
the credential record if one is set, and copies any `extra_env`
switches into the child process environment before exec. The bundled
`agentd` runtime then reads the value via `os.Getenv`.

## Provider configuration via Agentfile

Agentfiles bind a model with the `MODEL` directive:

```text
MODEL openai default base_url=https://api.openai.com/v1 auth=api_key api_key_env=OPENAI_API_KEY
MODEL anthropic default base_url=https://api.anthropic.com auth=api_key api_key_env=ANTHROPIC_API_KEY
MODEL vllm local base_url=http://localhost:8000/v1 auth=none
```

Combine with `FROM ./base/Agentfile` to share model defaults across many
agents.
