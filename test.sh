#!/usr/bin/env bash
# test.sh — run build.sh, then all tests (unit, validation, fuzz smoke, benchmarks).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

log() { printf '\033[1;34m[test]\033[0m %s\n' "$*"; }

"$ROOT/build.sh"

log "unit + validation tests"
go test ./... -count=1

# Fuzz smoke: run each fuzz target for FUZZ_TIME (default 3s) so the suite
# validates but doesn't hang forever in CI.
FUZZ_TIME="${FUZZ_TIME:-3s}"

log "fuzz smoke (${FUZZ_TIME} per target)"
FUZZ_PKG="./testing/fuzz/..."
if go test -list='^Fuzz' $FUZZ_PKG >/dev/null 2>&1; then
  while IFS= read -r target; do
    [ -n "$target" ] || continue
    log "  fuzzing $target"
    go test $FUZZ_PKG -run='^$' -fuzz="^${target}$" -fuzztime="$FUZZ_TIME"
  done < <(go test -list='^Fuzz' $FUZZ_PKG | grep -E '^Fuzz' || true)
fi

log "benchmarks (short)"
go test ./testing/benchmarks/... -run='^$' -bench=. -benchtime=1x -count=1

log "tests complete"
