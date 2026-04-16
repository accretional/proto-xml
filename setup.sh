#!/usr/bin/env bash
# setup.sh — install toolchain deps and generate Go code from .proto files.
# Idempotent: safe to run multiple times. Re-runs are skipped when outputs are up to date.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

log() { printf '\033[1;34m[setup]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[setup]\033[0m %s\n' "$*" >&2; }
die() { printf '\033[1;31m[setup]\033[0m %s\n' "$*" >&2; exit 1; }

# --- tool checks --------------------------------------------------------------

command -v go >/dev/null 2>&1 || die "go not found in PATH; install Go 1.26"
GO_VERSION="$(go env GOVERSION)"
case "$GO_VERSION" in
  go1.26*) : ;;
  *) die "Go 1.26 required, got $GO_VERSION" ;;
esac
log "go: $GO_VERSION"

command -v protoc >/dev/null 2>&1 || die "protoc not found in PATH; install protoc"
log "protoc: $(protoc --version)"

GOBIN="$(go env GOBIN)"
if [ -z "$GOBIN" ]; then
  GOBIN="$(go env GOPATH)/bin"
fi
export PATH="$GOBIN:$PATH"

if ! command -v protoc-gen-go >/dev/null 2>&1; then
  log "installing protoc-gen-go"
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi
log "protoc-gen-go: $(command -v protoc-gen-go)"

# --- go module ----------------------------------------------------------------

if [ ! -f go.mod ]; then
  log "initializing go module 'openformat'"
  # Module name matches the go_package prefix in xml.proto (openformat/gen/go/...),
  # so protoc output lands inside this module without rewriting.
  go mod init openformat
fi

# --- proto codegen ------------------------------------------------------------

PROTO_SRC_DIR="$ROOT/proto"
PROTO_FILES=(
  "openformat/v1/mime.proto"
  "openformat/v1/xml.proto"
)
GEN_DIR="$ROOT/gen/go/openformat/v1"

needs_regen=0
if [ ! -d "$GEN_DIR" ]; then
  needs_regen=1
else
  for pf in "${PROTO_FILES[@]}"; do
    src="$PROTO_SRC_DIR/$pf"
    base="$(basename "$pf" .proto)"
    out="$GEN_DIR/${base}.pb.go"
    if [ ! -f "$out" ] || [ "$src" -nt "$out" ]; then
      needs_regen=1
      break
    fi
  done
fi

if [ "$needs_regen" -eq 1 ]; then
  log "regenerating protobuf Go sources"
  mkdir -p "$GEN_DIR"
  rm -f "$GEN_DIR"/*.pb.go
  protoc \
    --proto_path="$PROTO_SRC_DIR" \
    --go_out="$ROOT" \
    --go_opt=module=openformat \
    "${PROTO_FILES[@]/#/$PROTO_SRC_DIR/}"
else
  log "proto outputs up to date"
fi

# --- go deps ------------------------------------------------------------------

log "resolving go dependencies"
go mod tidy

log "setup complete"
