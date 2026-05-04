# Install

## Requirements

- Go 1.26.2 or newer for source builds.
- A POSIX shell for the current sample local-process agents.
- Optional packaging tools for local release builds: `zip`, `tar`, `dpkg-deb`, `rpmbuild`, and `sha256sum`.

## From Source

```bash
git clone https://github.com/invariantcontinuum/agentctl.git
cd agentctl
make build
sudo install -m 0755 bin/agentctl /usr/local/bin/agentctl
sudo install -m 0755 bin/agentd   /usr/local/bin/agentd
```

`make build` produces both the CLI (`bin/agentctl`) and the bundled
runtime (`bin/agentd`). `agentctl run` defaults `EXEC` to `agentd` when
the Agentfile omits its own command, so installing both ensures the
out-of-the-box `run` flow works without configuration.

## Release Assets

Release automation is configured to publish:

- Windows x64 zip.
- Windows ARM64 zip.
- macOS x64 tarball.
- macOS ARM64 tarball.
- Linux x64 tarball.
- Linux ARM64 tarball.
- Debian/Ubuntu x64 `.deb`.
- Debian/Ubuntu ARM64 `.deb`.
- RHEL/Fedora x64 `.rpm`.
- RHEL/Fedora ARM64 `.rpm`.
- Checksums.
- Release SBOM.

Release publishing is configured in `.github/workflows/release.yml`.

## One-Line Install Examples

Set the repository once for shell examples:

```bash
export AGENTCTL_REPO=invariantcontinuum/agentctl
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

macOS x64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_darwin_amd64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

macOS ARM64:

```bash
TAG=$(curl -fsSL "https://api.github.com/repos/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest" | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p') && VERSION=${TAG#v} && curl -fsSL "https://github.com/${AGENTCTL_REPO:-invariantcontinuum/agentctl}/releases/latest/download/agentctl_${VERSION}_darwin_arm64.tar.gz" | sudo tar -xz -C /usr/local/bin agentctl
```

Windows x64 PowerShell:

```powershell
$r=Invoke-RestMethod "https://api.github.com/repos/invariantcontinuum/agentctl/releases/latest"; $v=$r.tag_name.TrimStart("v"); New-Item -ItemType Directory -Force "$env:USERPROFILE\bin" | Out-Null; Invoke-WebRequest "https://github.com/invariantcontinuum/agentctl/releases/latest/download/agentctl_${v}_windows_amd64.zip" -OutFile "$env:TEMP\agentctl.zip"; Expand-Archive "$env:TEMP\agentctl.zip" -DestinationPath "$env:USERPROFILE\bin" -Force
```

Windows ARM64 PowerShell:

```powershell
$r=Invoke-RestMethod "https://api.github.com/repos/invariantcontinuum/agentctl/releases/latest"; $v=$r.tag_name.TrimStart("v"); New-Item -ItemType Directory -Force "$env:USERPROFILE\bin" | Out-Null; Invoke-WebRequest "https://github.com/invariantcontinuum/agentctl/releases/latest/download/agentctl_${v}_windows_arm64.zip" -OutFile "$env:TEMP\agentctl.zip"; Expand-Archive "$env:TEMP\agentctl.zip" -DestinationPath "$env:USERPROFILE\bin" -Force
```
