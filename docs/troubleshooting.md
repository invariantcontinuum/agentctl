# Troubleshooting

## `agentctl run` Cannot Find `Agentfile`

By default, `agentctl run` reads `Agentfile` in the current directory.

Use `-f` to specify another path:

```bash
agentctl run -f ./path/to/Agentfile
```

## `rm` Refuses a Running Agent

This is intentional Docker-like behavior.

Stop first:

```bash
agentctl stop <agent-id>
agentctl rm <agent-id>
```

Or force removal:

```bash
agentctl rm -f <agent-id>
```

## `ps` Shows No Agents

`agentctl ps` only shows running agents.

Use:

```bash
agentctl ps -a
agentctl ps -aq
```

## State Appears in the Wrong Place

Check XDG environment variables:

```bash
echo "$XDG_CONFIG_HOME"
echo "$XDG_CACHE_HOME"
```

The state file is under the platform config directory at `agentctl/state.json`.

## `agentctl run` Cannot Find `agentd`

When an Agentfile omits `EXEC`, `agentctl run` injects `agentd` as the default
runtime. Build and install both binaries for that flow:

```bash
make build
sudo install -m 0755 bin/agentctl /usr/local/bin/agentctl
sudo install -m 0755 bin/agentd   /usr/local/bin/agentd
```

Alternatively, declare an explicit `EXEC` command in the Agentfile.

## Release Packaging Fails Locally

Full release packaging needs `rpmbuild`.

Install RPM tooling for local `.rpm` builds, or rely on the release workflow, which installs the required packaging tools.

## Documentation Deploy Fails

The `Deploy Documentation` workflow installs `mkdocs-material`, runs
`mkdocs build --strict`, and deploys the generated `site/` artifact through
GitHub Pages. Check:

- GitHub Pages is configured to use GitHub Actions as the source.
- The workflow has `pages: write` and `id-token: write` permissions.
- `mkdocs.yml` navigation includes every referenced page.

## Model Credentials

`Agentfile` should reference credential environment variable names, not raw secrets.

Example:

```text
MODEL openai default base_url=https://api.openai.com/v1 auth=api_key api_key_env=OPENAI_API_KEY
```
