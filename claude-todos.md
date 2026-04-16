# claude-todos

Follow-up work beyond the README's initial checklist, ordered roughly by
value-to-risk. Everything here is concrete enough to implement; the fuzzier
"someday" items at the bottom are noted as such.

## 1. Attribute-value entity/char-ref fidelity — **done**

`XmlAttribute.LiteralValue` is now populated by pre-scanning the raw
start-tag bytes (`extractAttrLiterals` in `internal/xmlcodec/decode.go`).
The encoder emits from the literal pieces when present, preserving both
the entity/char-ref form and the original quote character. Fixture:
`data/handwritten/16_attr_entity_refs.xml`. Unit test:
`TestAttrLiteralRoundTrip` asserts byte-identical structural round-trip.

## 2. Real chromerpc screenshots — **done**

PNGs under `docs/screenshots/` are now real captures taken via a local
chromerpc server driving headless Chrome against the `_html/` demo pages
(commit `c90eab3`). `cmd/demo-screenshots` still falls back to placeholder
PNGs when `CHROMERPC_ADDR` is unreachable.

## 3. Charset transcoding — **todo**

**Problem.** `CharsetReader` is a no-op. A document declared as UTF-16 or
`windows-1252` but actually not UTF-8 will parse to garbage (or error) silently.

**Plan.**
- Add `golang.org/x/net/html/charset` as a dep (tiny; already implied by
  most Go XML stacks).
- Wire `dec.CharsetReader = charset.NewReaderLabel`.
- Add a fixture written in UTF-16LE with a BOM; extend validation to cover it.
- Document in `testing/README.md` which encodings are reachable.

## 4. DOCTYPE internal subset — **todo (larger)**

**Problem.** `XmlDocumentTypeDeclaration.internal_subset` is empty today. The
proto has full types for element / attlist / entity / notation declarations.

**Plan.**
- Hand-roll a DTD internal-subset parser that runs on the raw bytes between
  `[` and `]` inside the DOCTYPE. Output populated
  `XmlInternalSubsetDeclaration` oneofs.
- Encoder re-emits them in order.
- Scope out parameter entity expansion — declare it unsupported in
  `testing/README.md` and stop there unless real users need it.

**Verification.** Add fixture with inline `<!ENTITY>`, `<!ELEMENT>`,
`<!ATTLIST>`; round-trip via structural encode.

## 5. Streaming / iterator API — **todo (speculative)**

Only worth doing if someone shows up with >10MB feeds. Current API holds the
whole document in memory. Rough plan: add `DecodeStream(io.Reader, func(Event))`
emitting Element Start/End and text events without building the full tree.
Don't start this without a user asking.

## 6. C14N (canonical XML) output — **todo (speculative)**

The proto already has `XmlCanonicalizationConfig`. Implementing C14N 1.0
(exclusive/inclusive, with/without comments) is a well-defined spec exercise
but nontrivial. Defer until someone needs deterministic hashing.

## 7. Benchmark tuning — **todo (low priority)**

Current `BenchmarkRoundTrip` shows 60–900 allocs per fixture — mostly proto
message allocations that are hard to pool without breaking proto semantics.
Investigate whether a node-allocation arena helps before committing.

---

**Working order.** Items 1 and 2 complete. Next up: 3 (charset
transcoding); 4 pulled in if the user asks. Everything below 4 stays
queued until there's concrete demand.
