#!/usr/bin/env bash
# LET_IT_RIP.sh — top-level "ship it" entry point. Runs full test suite.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

log() { printf '\033[1;32m[LET_IT_RIP]\033[0m %s\n' "$*"; }

"$ROOT/test.sh"

log "all systems go"
