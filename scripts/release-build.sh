#!/usr/bin/env bash
set -euo pipefail

version="${1:?usage: scripts/release-build.sh <version> [dist-dir]}"
dist_dir="${2:-dist}"
version="${version#v}"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must be SemVer without prerelease metadata: $version" >&2
  exit 1
fi

need_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "required command is missing: $command_name" >&2
    exit 127
  fi
}

need_command go
need_command tar
need_command zip
need_command dpkg-deb
need_command rpmbuild
need_command sha256sum

rm -rf "$dist_dir"
mkdir -p "$dist_dir"
dist_dir="$(cd "$dist_dir" && pwd)"

build_root="$(mktemp -d)"
trap 'rm -rf "$build_root"' EXIT

build_binary() {
  local goos="$1"
  local goarch="$2"
  local output="$3"

  mkdir -p "$(dirname "$output")"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags="-s -w" -o "$output" ./cmd/agentctl
}

archive_binary() {
  local goos="$1"
  local goarch="$2"

  local ext=""
  if [[ "$goos" == "windows" ]]; then
    ext=".exe"
  fi

  local binary_dir="$build_root/archive/${goos}_${goarch}"
  local binary="$binary_dir/agentctl${ext}"
  build_binary "$goos" "$goarch" "$binary"

  if [[ "$goos" == "windows" ]]; then
    (cd "$binary_dir" && zip -q "$dist_dir/agentctl_${version}_${goos}_${goarch}.zip" "agentctl${ext}")
  else
    tar -C "$binary_dir" -czf "$dist_dir/agentctl_${version}_${goos}_${goarch}.tar.gz" agentctl
  fi
}

build_deb() {
  local goarch="$1"
  local deb_arch="$2"
  local root="$build_root/deb/${goarch}"

  build_binary linux "$goarch" "$root/usr/bin/agentctl"
  chmod 0755 "$root/usr/bin/agentctl"
  mkdir -p "$root/DEBIAN"
  cat >"$root/DEBIAN/control" <<CONTROL
Package: agentctl
Version: $version
Section: utils
Priority: optional
Architecture: $deb_arch
Maintainer: Invariant Continuum <maintainers@invariantcontinuum.io>
Description: Docker-style control-plane CLI for long-running AI agents.
CONTROL
  dpkg-deb --build "$root" "$dist_dir/agentctl_${version}_linux_${goarch}.deb" >/dev/null
}

build_rpm() {
  local goarch="$1"
  local rpm_arch="$2"
  local top="$build_root/rpm/${goarch}"
  local source_dir="$top/SOURCES/agentctl-${version}"

  mkdir -p "$top/BUILD" "$top/RPMS" "$top/SOURCES" "$top/SPECS" "$top/SRPMS" "$source_dir/usr/bin"
  build_binary linux "$goarch" "$source_dir/usr/bin/agentctl"
  chmod 0755 "$source_dir/usr/bin/agentctl"
  tar -C "$top/SOURCES" -czf "$top/SOURCES/agentctl-${version}.tar.gz" "agentctl-${version}"

  cat >"$top/SPECS/agentctl.spec" <<SPEC
Name: agentctl
Version: $version
Release: 1%{?dist}
Summary: Docker-style control-plane CLI for long-running AI agents
License: LicenseRef-UNSPECIFIED
BuildArch: $rpm_arch
Source0: %{name}-%{version}.tar.gz

%description
agentctl manages long-running AI agents from Agentfile manifests.

%prep
%setup -q

%build

%install
mkdir -p %{buildroot}/usr/bin
install -m 0755 usr/bin/agentctl %{buildroot}/usr/bin/agentctl

%files
/usr/bin/agentctl
SPEC

  rpmbuild --define "_topdir $top" -bb "$top/SPECS/agentctl.spec" >/dev/null
  cp "$top/RPMS/$rpm_arch/agentctl-${version}-1.$rpm_arch.rpm" "$dist_dir/agentctl_${version}_linux_${rpm_arch}.rpm"
}

archive_binary linux amd64
archive_binary linux arm64
archive_binary darwin amd64
archive_binary darwin arm64
archive_binary windows amd64
archive_binary windows arm64

build_deb amd64 amd64
build_deb arm64 arm64

build_rpm amd64 x86_64
build_rpm arm64 aarch64

(cd "$dist_dir" && sha256sum * > checksums.txt)
