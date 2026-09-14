#!/usr/bin/env bash
# M1 acceptance: envorca start -> status -> stop round-trips through the
# daemon over local IPC. Works on Linux (Unix socket) and Windows CI shells.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

export ENVORCA_SOCKET="$tmpdir/envorca.sock"

( cd "$root/daemon" && go build -o "$tmpdir/envorcad" ./cmd/envorcad )
( cd "$root/cli" && go build -o "$tmpdir/envorca" ./cmd/envorca )

export PATH="$tmpdir:$PATH"

"$tmpdir/envorca" start
"$tmpdir/envorca" status

if ! "$tmpdir/envorca" status | grep -q "ENVORCA DAEMON"; then
  echo "FAILED: status output missing ENVORCA DAEMON header" >&2
  exit 1
fi
echo "STATUS ROUND-TRIP OK"

"$tmpdir/envorca" stop
sleep 1

if "$tmpdir/envorca" status >/dev/null 2>&1; then
  echo "FAILED: daemon still reachable after stop" >&2
  exit 1
fi
echo "STOP VERIFIED"