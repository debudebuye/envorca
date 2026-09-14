#!/usr/bin/env bash
# M1 acceptance: envorka start -> status -> stop round-trips through the
# daemon over local IPC. Works on Linux (Unix socket) and Windows CI shells.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

export ENVORKA_SOCKET="$tmpdir/envorka.sock"

( cd "$root/daemon" && go build -o "$tmpdir/envorkad" ./cmd/envorkad )
( cd "$root/cli" && go build -o "$tmpdir/envorka" ./cmd/envorka )

export PATH="$tmpdir:$PATH"

"$tmpdir/envorka" start
"$tmpdir/envorka" status

if ! "$tmpdir/envorka" status | grep -q "ENVORKA DAEMON"; then
  echo "FAILED: status output missing ENVORKA DAEMON header" >&2
  exit 1
fi
echo "STATUS ROUND-TRIP OK"

"$tmpdir/envorka" stop
sleep 1

if "$tmpdir/envorka" status >/dev/null 2>&1; then
  echo "FAILED: daemon still reachable after stop" >&2
  exit 1
fi
echo "STOP VERIFIED"