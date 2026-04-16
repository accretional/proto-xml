# Parsing

All XML parsing lives in `xmlcodec/decode.go`. There's no
dedicated parser module — `Decode(src []byte)` is the full pipeline.
This document maps the three layers the pipeline uses, the reason each
layer exists, and where to cut the seam if you want to swap in a
different parser.

## The pipeline, top-down

```
Decode(src) ─┐
             ├─► 2. byte-level pre-scans  (transcode, patch, sniff)
             ├─► 1. stdlib encoding/xml   (token stream)
             └─► 3. hand-rolled scans     (compensate for lossy tokens)
                 → XmlDocumentWithMetadata
```

Layers 2 and 3 exist only because layer 1 loses information we need for
byte-faithful round-trip.

## Layer 1 — `encoding/xml` tokenizer

The stdlib `xml.Decoder` does the heavy lifting: it emits
`StartElement`, `EndElement`, `CharData`, `Comment`, `ProcInst`, and
`Directive` tokens. We walk those in the main `for { dec.Token() }`
loop and translate each one into a proto node.

What the stdlib gives us for free: well-formedness checking, namespace
resolution, entity/character-reference resolution in both text and
attribute values, CDATA text content (merged into plain `CharData`),
and a `dec.InputOffset()` cursor into the byte stream.

What it won't do for us:
- Accept `version="1.1"`. Hard-coded to 1.0.
- Preserve CDATA boundaries — CDATA text is indistinguishable from
  plain character data in the token stream.
- Preserve the literal form of entity / character references. By the
  time we see a `CharData` or a `StartElement.Attr`, `&amp;` has
  already become `&`.
- Distinguish `<foo/>` from `<foo></foo>`. Both synthesise an
  `EndElement` token.
- Transcode non-UTF-8 byte streams without a user-supplied
  `CharsetReader`.
- Parse the DTD internal subset.

## Layer 2 — byte-level pre-scans

These run on `src` (or a copy) before we hand bytes to the stdlib
decoder. They each address one specific stdlib gap.

| Function | Purpose |
| --- | --- |
| `transcodeToUTF8` | Convert UTF-16 / windows-1252 / etc. to UTF-8 via `golang.org/x/net/html/charset`. Byte-offset scanning downstream is single-buffer. |
| `rewriteDeclaredEncoding` | After transcoding, rewrite `encoding="UTF-16"` → `encoding="UTF-8"` so stdlib's declaration check passes. |
| `patchXMLVersion` | Rewrite `version="1.1"` → `"1.0"` in the declaration (length-preserving) so stdlib accepts the stream. The original version is captured separately. |
| `detectEncoding` | BOM sniffing + declaration scan → `XmlEncodingInfo`. |
| `applyXMLDeclaration` / `parsePseudoAttrs` | Regex-based read of `version`, `encoding`, `standalone` from the declaration. Runs on the transcoded buffer *before* `rewriteDeclaredEncoding`, so the original declared encoding is preserved on `doc.CharacterEncodingScheme`. |

All three mutations (`transcodeToUTF8`, `rewriteDeclaredEncoding`,
`patchXMLVersion`) compose cleanly because each operates on the output
of the previous step, and the original bytes survive in
`XmlDocumentWithMetadata.RawBytes`.

## Layer 3 — hand-rolled mini-tokenizers

These run against the *parsed* (post-transcode, post-patch) byte buffer
at specific token boundaries supplied by stdlib. Each one reconstructs
information the stdlib tokenizer dropped.

| Function | Runs where | Reconstructs |
| --- | --- | --- |
| `scanCDATARanges` | Once, up-front | Byte ranges of `<![CDATA[...]]>`. Matched against CharData token ranges to classify nodes. |
| `scanTextPieces` | Per CharData token | Splits the raw bytes back into `character_data` / `entity_ref_name` / `char_ref_codepoint` pieces with hex/decimal flag. |
| `extractAttrLiterals` + `splitAttrValueIntoPieces` | Per StartElement token | Walks the raw start-tag bytes and re-parses each attribute's value into literal pieces + original quote character. |
| `parseDoctypeDirective` | Per Directive token | Extracts DOCTYPE name + PUBLIC/SYSTEM IDs. Internal subset is flagged but not parsed. |
| `isSelfClosingTag` | Per EndElement token when element has no children | Regex scan over the source for `<name .../>` to decide `self_closing=true`. |

These are all small, linear scans over bounded byte ranges — except
`isSelfClosingTag`, which scans the full source per element (O(n·m) in
the worst case).

## The integration seam

The public surface is one function:

```go
func Decode(src []byte) (*pb.XmlDocumentWithMetadata, error)
```

and one proto shape (defined in `proto/openformat/v1/xml.proto`). If you
swap in a different parser — for example, an advanced one from another
project — it replaces layers 1–3 wholesale. The contract with
`xmlcodec/encode.go` is purely the proto:

- `XmlElement.InScopeNamespaces` must be populated for every element if
  you want the structural encoder to resolve attribute prefixes.
- `XmlElement.SelfClosing` controls `<foo/>` vs `<foo></foo>`.
- `XmlElement.QualifiedName` is preferred over `Prefix + LocalName` at
  encode time.
- `XmlText.RawPieces` and `XmlAttribute.LiteralValue.Pieces`, when set,
  take precedence over `Data` / `NormalizedValue` and preserve entity /
  char-ref form.
- `XmlAttributeValueLiteral.QuoteChar` picks single vs double quotes on
  re-emit.
- `XmlDocumentWithMetadata.RawBytes` must hold the original source for
  the raw-bytes encode path to work.

A replacement parser that produces the same proto shape is a drop-in.

## Known weaknesses

- `isSelfClosingTag` is O(n·m) and can misclassify pathological inputs
  where a comment contains text that looks like the tag literal.
- DOCTYPE internal subset is unparsed — only name + external IDs
  survive a structural re-encode. Raw-bytes encode is unaffected.
- XML 1.1-only name-character classes aren't validated (we parse as
  1.0 because stdlib does).
- The hand-rolled tokenizers in layer 3 (`extractAttrLiterals`,
  `splitAttrValueIntoPieces`, `scanTextPieces`) are where fuzz-found
  hangs have historically originated. `go test -fuzz=.` against
  `testing/fuzz` is the guard rail.

## See also

- `testing/README.md` — full list of semantic discrepancies.
- `README.md` `## NEXT STEPS` — running findings about the format / proto.
- `claude-todos.md` — queued follow-ups (DOCTYPE subset, streaming, C14N).
