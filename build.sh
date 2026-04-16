#!/usr/bin/env bash
# build.sh — run setup.sh, then compile the project.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

log() { printf '\033[1;34m[build]\033[0m %s\n' "$*"; }

"$ROOT/setup.sh"

log "go vet ./..."
go vet ./...

log "go build ./..."
go build ./...

log "build complete"
