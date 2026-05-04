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

## Release Packaging Fails Locally

Full release packaging needs `rpmbuild`.

Install RPM tooling for local `.rpm` builds, or rely on the release workflow, which installs the required packaging tools.

## Sonar Scan Fails in CI

Configure:

- `SONAR_TOKEN` as a repository secret.
- `SONAR_HOST_URL` as a repository variable when using SonarQube Server instead of SonarCloud.

## Model Credentials

`Agentfile` should reference credential environment variable names, not raw secrets.

Example:

```text
MODEL openai default endpoint=https://api.openai.com/v1 auth=api_key credential_env=OPENAI_API_KEY
```
