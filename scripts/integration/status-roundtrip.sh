#!/usr/bin/env bash
# M1 acceptance: runorka start -> status -> stop round-trips through the
# daemon over local IPC. Works on Linux (Unix socket) and Windows CI shells.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

export RUNORKA_SOCKET="$tmpdir/runorka.sock"

( cd "$root/daemon" && go build -o "$tmpdir/runorkad" ./cmd/runorkad )
( cd "$root/cli" && go build -o "$tmpdir/runorka" ./cmd/runorka )

export PATH="$tmpdir:$PATH"

"$tmpdir/runorka" start
"$tmpdir/runorka" status

if ! "$tmpdir/runorka" status | grep -q "RUNORKA DAEMON"; then
  echo "FAILED: status output missing RUNORKA DAEMON header" >&2
  exit 1
fi
echo "STATUS ROUND-TRIP OK"

"$tmpdir/runorka" stop
sleep 1

if "$tmpdir/runorka" status >/dev/null 2>&1; then
  echo "FAILED: daemon still reachable after stop" >&2
  exit 1
fi
echo "STOP VERIFIED"