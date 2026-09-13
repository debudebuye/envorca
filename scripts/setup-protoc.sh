#!/usr/bin/env bash
# Installs a pinned protoc for CI and contributors. Set PROTOC_VERSION and
# PROTOC_PREFIX to override. Plugins (protoc-gen-go, protoc-gen-go-grpc) are
# installed separately with `go install`.
set -euo pipefail

VERSION="${PROTOC_VERSION:-36.1}"
PREFIX="${PROTOC_PREFIX:-$HOME/.local}"

os="$(uname -s | tr 'A-Z' 'a-z')"
case "$os" in
  linux) os="linux" ;;
  darwin) os="osx" ;;
  *) echo "unsupported os: $os" >&2; exit 1 ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="x86_64" ;;
  aarch64 | arm64) arch="aarch_64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

url="https://github.com/protocolbuffers/protobuf/releases/download/v${VERSION}/protoc-${VERSION}-${os}-${arch}.zip"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "installing protoc ${VERSION} -> ${PREFIX}/bin"
curl -fsSL "$url" -o "$tmp/protoc.zip"
unzip -o -q "$tmp/protoc.zip" -d "$PREFIX"
"$PREFIX/bin/protoc" --version