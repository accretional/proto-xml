# proto-xml

## Instructions

Make sure you create a setup.sh, build.sh, test.sh, and LET_IT_RIP.sh that contain all project setup scripts/commands used - NEVER build/test/run the code in this repo outside of these scripts, NEVER commit or push without running these either. Make them idempotent so that each build.sh can run setup.sh and skip things already set up, each test.sh can run build.sh, each LET_IT_RIP runs test.sh

use go1.26

1. Import https://github.com/accretional/mime-proto/blob/main/pb/proto/openformat/v1/xml.proto to xml.proto and related xml-specific logic
2. Make sure the encoder/decoder use cases work fully e2e with unit tests
3. Create data/ directory with multiple xml files exhibiting various different aspects of the format that we can use for testing. Create some programmatically using the protos and others just as regular old xml files.
4. Create a testing/validation/ directory running a suite of tests (for now just one) across all the data/
5. Create a testing/fuzz/ directory running fuzzing tests
6. Create a testing/benchmarks directory running benchmarks across the data
7. Document any discrepancies or irregularities in the testing in testing/README.md, as well as the overall strategy/setup
8. Augment this README.md in ## NEXT STEPS with anything important you find, any irregularities in the file format, bad implementations, missing functionality, etc.
9. Write a docs/about.md explaining this project, with examples, in a way someone might actually use it (eg with rss). Use github.com/accretional/chromerpc to take screenshots as you walk through a demo of a real xml file. Prepare to embed these images in about.md in the github markdown format.

## NEXT STEPS

Findings surfaced while implementing the codec and test suite. Update this
list as new ones turn up.

### Upstream proto (`accretional/mime-proto`, `openformat/v1/xml.proto`)

- `XmlText.raw_pieces` is repeated `XmlTextPiece`, where each piece is a
  oneof of `character_data | entity_ref_name | char_ref_codepoint |
  char_ref_is_hex`. The hex flag being its own piece (instead of a field
  on `char_ref_codepoint`) is awkward: the encoder has to emit the
  codepoint piece, then later see the hex marker piece and retroactively
  rewrite the previous output. We currently emit decimal and ignore the
  hex marker on the structural path — a faithful re-emit needs the proto
  to bind hex-vs-decimal to the codepoint itself.
- `XmlElement.in_scope_namespaces` is a `map<string,string>` (prefix→URI).
  Map iteration in protobuf is non-deterministic, so re-emitting in
  source-order requires keeping a parallel ordered list. Consider adding
  a repeated `XmlNamespaceBinding { prefix, uri }` field for stable order.
- `XmlAttribute` has no `prefix` field — only `namespace_name` and
  `local_name`. The encoder has to look the prefix up from the element's
  in-scope namespaces every time it writes an attribute. A `prefix`
  string on `XmlAttribute` would simplify both ends and let us preserve
  the source's prefix choice when several prefixes bind the same URI.
- `XmlDocumentTypeDeclaration` doesn't model the internal subset at all
  (`<!ENTITY ... >`, `<!ELEMENT ... >`, `<!ATTLIST ... >`). It's currently
  represented only by a `has_internal_subset` bool — anyone wanting to
  re-emit must keep the raw bytes.
- `XmlMiscNode` is a oneof of `processing_instruction | comment` only —
  no place for whitespace between prolog/epilog items. Consequence: the
  structural encode path collapses inter-misc whitespace.
- No fields for XML 1.1 specifics (e.g. `<?xml version="1.1" ?>`-only
  name-character classes, NEL line endings). We round-trip the version
  enum but not the semantics.

### Format quirks observed in fixtures

- The Go stdlib `encoding/xml` package only accepts `version="1.0"`. We
  patch `1.1` → `1.0` in-memory just to get the bytes through the parser
  (see `patchXMLVersion`). Anything that depends on 1.1-only character
  rules will silently parse as 1.0.
- Entity / character references inside attribute values are resolved by
  stdlib before we see the token. To preserve them we pre-scan the raw
  start-tag bytes and populate `XmlAttribute.LiteralValue` (pieces +
  original quote character). Structural encode emits from the literal
  form when present, so `foo="&amp;"` no longer round-trips to
  `foo="&"`.
- `xml:space="preserve"` is captured on the element but the structural
  encoder doesn't currently use it as a hint for whether to elide
  element-content whitespace text nodes. (Today we never elide, so
  behaviour is correct by accident — flagging in case future encoder
  optimisations skip whitespace.)

### Codec implementation gaps

- No streaming Decode/Encode — the whole document is held in memory.
  Acceptable for typical XML payloads, problematic for multi-MB feeds.
- Non-UTF-8 inputs (UTF-16 with BOM, windows-1252, etc.) are transcoded
  to UTF-8 upfront via `golang.org/x/net/html/charset` so byte-offset
  scanning (CDATA, attribute literals, text pieces) works on a single
  buffer. Original bytes survive in `RawBytes`. Structural encode always
  emits UTF-8 regardless of source encoding.
- Self-closing detection (`isSelfClosingTag`) is a regex scan over the
  full source per element. O(n·m). Fine for fixtures, replace with offset
  capture during decode for production use.
