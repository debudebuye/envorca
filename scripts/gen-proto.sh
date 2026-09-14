#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
proto_file="$root/api/proto/envorca/v1/envorca.proto"
go_out="$root/api/gen"

protoc -I "$root/api/proto" \
  --go_out="$go_out" --go_opt=module=envorca.dev/envorca/api/gen \
  --go-grpc_out="$go_out" --go-grpc_opt=module=envorca.dev/envorca/api/gen \
  "$proto_file"

echo "generated: $(cd "$go_out" && find go -name '*.go' | sort)"