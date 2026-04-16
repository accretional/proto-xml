# CLAUDE.md

See `AGENTS.md` — same rules apply. This file is specifically for Claude Code sessions.

## Quick reference

- Go toolchain: `go1.26` (box has `go1.26.2`).
- Build/test ONLY via `./setup.sh`, `./build.sh`, `./test.sh`, `./LET_IT_RIP.sh`. Each wraps the previous. All idempotent.
- Never run `go test ./...` or `go build ./...` directly for CI-style validation — use the scripts.
- Before committing or pushing: `./LET_IT_RIP.sh` must pass.

## Proto

- Source: private repo `accretional/mime-proto`, file `pb/proto/openformat/v1/xml.proto`. Also pulls `mime.proto` (same dir).
- Vendored to `proto/openformat/v1/` in this repo. Regenerate with `./setup.sh`.
- Generated Go lives at `gen/go/openformat/v1/` (package `openformatv1`).
- Proto has `go_package = "openformat/gen/go/openformat/v1;openformatv1"`, so `protoc --go_opt=module=<mod>` is used to resolve the path inside this module.

## Code layout

- `internal/xmlcodec/` — encoder (proto → XML bytes) and decoder (XML bytes → proto). `XmlDocumentWithMetadata.raw_bytes` is always set by Decode and required by Encode for round-trip fidelity.
- `data/` — XML fixtures covering: namespaces, CDATA, comments, processing instructions, DOCTYPE/DTD, entities, character references, `xml:space`/`xml:lang`/`xml:id`/`xml:base`, mixed content, XInclude, BOMs, XML 1.0 vs 1.1. Some are hand-written, others generated from `XmlDocument` protos.
- `testing/validation/` — one test running across every file in `data/`.
- `testing/fuzz/` — Go native fuzz tests (`go test -fuzz`).
- `testing/benchmarks/` — `Benchmark*` functions run across `data/`.

## Documentation outputs

- `README.md` `## NEXT STEPS` — append findings (format quirks, missing features, bugs in upstream proto).
- `testing/README.md` — overall test strategy + any discrepancies.
- `docs/about.md` — narrative with a worked example (RSS is a natural fit) and screenshots via `github.com/accretional/chromerpc` (gRPC client against a running chromerpc server at `localhost:50051`).

## ChromeRPC usage

`chromerpc` is a Go gRPC client for Chrome DevTools Protocol. Dial `localhost:50051`, then send `AutomationSequence` with `navigate` + `screenshot` steps. Assume a server is running; if not, the screenshot generation step fails gracefully and `about.md` references placeholder paths.
