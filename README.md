# agentctl

## Description

`agentctl` is a Docker-style control-plane CLI for long-running AI agents. It reads deterministic `Agentfile` manifests, starts and manages agent processes, stores local instance state, and exposes lifecycle commands that can grow into Docker, Podman, Kubernetes, or systemd-backed drivers.

The project is Go 1.26.2+, standard-library-first, and organized around explicit package boundaries for domain validation, `Agentfile` parsing, state persistence, runtime drivers, and CLI presentation.

## Install

Release assets are published for Windows, macOS, Linux tarballs, Debian/Ubuntu `.deb`, and RHEL/Fedora `.rpm` on x64 and ARM64.

Set the repository once for shell examples:

```bash
export AGENTCTL_REPO=invariantcontinuum/agentctl
```

Windows x64 PowerShell:

```powershell
$r=Invoke-RestMethod "https://api.github.com/repos/invariantcontinuum/agentctl/releases/latest"; $v=$r.tag_name.TrimStart("v"); New-Item -ItemType Directory -Force "$env:USERPROFILE\bin" | Out-Null; Invoke-WebRequest "https://github.com/invariantcontinuum/agentctl/releases/latest/download/agentctl_${v}_windows_amd64.zip" -OutFile "$env:TEMP\agentctl.zip"; Expand-Archive "$env:TEMP\agentctl.zip" -DestinationPath "$env:USERPROFILE\bin" -Force
```

Windows ARM64 PowerShell:

```powershell
$r=Invoke-RestMethod "https://api.github.com/repos/invariantcontinuum/agentctl/releases/latest"; $v=$r.tag_name.TrimStart("v"); New-Item -ItemType Directory -Force "$env:USERPROFILE\bin" | Out-Null; Invoke-WebRequest "https://github.com/invariantcontinuum/agentctl/releases/latest/download/agentctl_${v}_windows_arm64.zip" -OutFile "$env:TEMP\agentctl.zip"; Expand-Archive "$env:TEMP\agentctl.zip" -DestinationPath "$env:USERPROFILE\bin" -Force
```

macOS x64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_darwin_amd64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

macOS ARM64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_darwin_arm64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

Linux x64 tarball:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_amd64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

Linux ARM64 tarball:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_arm64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

Debian/Ubuntu x64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_amd64.deb" -o /tmp/agentctl.deb && sudo dpkg -i /tmp/agentctl.deb
```

Debian/Ubuntu ARM64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_arm64.deb" -o /tmp/agentctl.deb && sudo dpkg -i /tmp/agentctl.deb
```

RHEL/Fedora x64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_x86_64.rpm" -o /tmp/agentctl.rpm && sudo rpm -Uvh /tmp/agentctl.rpm
```

RHEL/Fedora ARM64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_linux_aarch64.rpm" -o /tmp/agentctl.rpm && sudo rpm -Uvh /tmp/agentctl.rpm
```

From source:

```bash
git clone https://github.com/invariantcontinuum/agentctl.git && cd agentctl && make build && sudo install -m 0755 bin/agentctl /usr/local/bin/agentctl
```

## Usage

Parse and validate an `Agentfile` without starting it:

```bash
agentctl run -f Agentfile --dry-run
```

Start the sample local planner agent:

```bash
agentctl run -f Agentfile
```

List known agents:

```bash
agentctl ps
```

Inspect an agent:

```bash
agentctl inspect planner-local-<suffix>
```

Read logs and trace:

```bash
agentctl logs planner-local-<suffix>
agentctl trace planner-local-<suffix>
```

Stop, start, or restart an agent:

```bash
agentctl stop planner-local-<suffix>
agentctl start planner-local-<suffix>
agentctl restart planner-local-<suffix>
```

List local skills and configured MCP tools:

```bash
agentctl list-skills ./skills
agentctl list-tools planner-local-<suffix>
```

## Agentfile

See [docs/agentfile.md](docs/agentfile.md) and the sample [Agentfile](Agentfile).

## Development

```bash
make fmt
make fmt-check
make lint
make test
make build
```

## CI and Releases

CI runs formatting checks, `go vet`, tests with coverage, build, SonarQube/SonarCloud analysis, Syft SBOM generation, and Grype CVE scanning.

Required repository configuration:

- Secret `SONAR_TOKEN` for SonarQube/SonarCloud analysis.
- Optional variable `SONAR_HOST_URL`; defaults can be handled by SonarCloud, but set it for SonarQube Server.

Releases can be created by pushing a `vX.Y.Z` tag or by manually running the release workflow with `major`, `minor`, or `patch` bump input. Release assets include checksums, SBOM, archives, `.deb`, and `.rpm` packages.
