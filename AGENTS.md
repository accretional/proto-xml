# AGENTS.md

Ground rules for agents (and humans) working in this repo. These are distilled from `README.md` — read that file for the source of truth.

## Cardinal rule: use the scripts

All build/test/run activity MUST go through these four scripts. Never invoke `go build`, `go test`, `protoc`, etc. directly from the CLI in the normal workflow.

- `setup.sh` — install toolchain, fetch deps, generate code from `.proto`. Idempotent.
- `build.sh` — runs `setup.sh`, then compiles everything. Idempotent.
- `test.sh` — runs `build.sh`, then every test (unit, validation, fuzz smoke, benchmarks).
- `LET_IT_RIP.sh` — runs `test.sh`. Top-level "ship it" entry point.

Each script is a superset of the previous: `LET_IT_RIP → test → build → setup`. Do NOT commit or push without running `LET_IT_RIP.sh` successfully.

## Toolchain

- Go 1.26 (`go1.26`). The local dev box has `go1.26.2`.
- `protoc` + `protoc-gen-go` for code generation.

## Layout

```
proto/openformat/v1/        vendored .proto sources (xml.proto + mime.proto)
gen/go/openformat/v1/       generated Go (go_package per proto: openformat/gen/go/openformat/v1;openformatv1)
internal/xmlcodec/          encoder + decoder
data/                       XML test fixtures (hand-written + programmatically generated)
testing/validation/         validation test suite across all data/
testing/fuzz/               fuzz tests
testing/benchmarks/         benchmarks
testing/README.md           strategy + discrepancies
docs/about.md               narrative docs + embedded screenshots
```

## Proto source

The `xml.proto` comes from `github.com/accretional/mime-proto` at `pb/proto/openformat/v1/xml.proto`. It imports `openformat/v1/mime.proto` from the same repo. Both are vendored into `proto/openformat/v1/`. Do not hand-edit generated code under `gen/`.

`xml.proto` has `option go_package = "openformat/gen/go/openformat/v1;openformatv1"`. The generated Go package is `openformatv1`; its import path inside this module is `<module>/gen/go/openformat/v1`.

## README responsibilities not yet automated

- `README.md` has a `## NEXT STEPS` section where agents must record format irregularities, missing functionality, and other findings surfaced during implementation/testing.
- `testing/README.md` must document the overall test strategy and any discrepancies/irregularities.
- `docs/about.md` must demonstrate a real use case (e.g. RSS) with screenshots generated via `github.com/accretional/chromerpc`.

## Style

- Keep comments minimal; use them only when the *why* is non-obvious.
- Prefer editing existing files over creating new ones.
- Do not introduce abstractions beyond what a task needs.
