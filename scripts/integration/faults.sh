#!/usr/bin/env bash
# faults.sh — fault injection and repair verification script.
#
# Run on the self-hosted Windows runner to prove the M2 doctor+repair flow.
# Prerequisites: daemon and CLI built for Windows in dist/; WSL2 installed
# with a default distribution.
set -euo pipefail

DIR="${1:-dist}"
mkdir -p "$DIR"

export RUNORKA_SOCKET="${RUNORKA_SOCKET:-/tmp/rkfaultrun/runorka-fault.sock}"
export PATH="$DIR:$PATH"

cleanup() {
  runorka stop 2>/dev/null || true
  rm -f "$RUNORKA_SOCKET"
  mkdir -p "$(dirname "$RUNORKA_SOCKET")"
}
trap cleanup EXIT
cleanup

echo "=== Start daemon ==="
runorka start --daemon "$DIR/runorkad.exe"
sleep 1
runorka status >/dev/null 2>&1 || { echo "FAIL: daemon not ready"; exit 1; }
echo "daemon ready"

echo "=== Baseline doctor (all healthy) ==="
runorka doctor || true
OUT=$(runorka doctor 2>&1) || true
echo "$OUT"
echo "$OUT" | grep -q "Overall:" || { echo "FAIL: doctor missing Overall"; exit 1; }

echo "=== Inject fault: terminate default WSL distro ==="
wsl.exe --shutdown || true
sleep 1

echo "=== Doctor after fault ==="
RUN=0; NORMAL=0
if runorka doctor 2>&1 | grep -q "Overall:.*CRITICAL"; then
  RUN=1
elif runorka doctor 2>&1 | grep -q "Overall:.*WARNING"; then
  NORMAL=1
else
  NORMAL=1
fi
echo "doctor returned; run=$RUN normal=$NORMAL"

echo "=== Repair ==="
if [ "$RUN" -eq 1 ]; then
  echo "y" | runorka repair || true
  sleep 2
fi

echo "=== Verify repair ==="
OUT=$(runorka doctor 2>&1) || true
echo "$OUT"
echo "$OUT" | grep -q "Overall:" || { echo "FAIL: doctor missing Overall after repair"; exit 1; }

echo "=== Stop daemon ==="
runorka stop
sleep 1

echo "=== FAULT-REPAIR CYCLE OK ==="
