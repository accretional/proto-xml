// Package xmlcodec converts between raw XML bytes and the openformat.v1
// XmlDocument / XmlDocumentWithMetadata protobuf types.
//
// Decode parses raw XML into a structured XmlDocumentWithMetadata, preserving
// prolog/epilog, comments, processing instructions, CDATA sections, element
// structure, attributes, namespace declarations, and character/entity
// references in text content.  raw_bytes is always set to a copy of the input
// so callers can do a byte-faithful re-emit.
//
// Encode writes an XmlDocument back to XML bytes.  The output is a structural
// re-emit, not guaranteed byte-identical to the input that produced it — for
// that, callers should use XmlDocumentWithMetadata.RawBytes.
//
// Spec fidelity is best-effort and deliberately scoped: see
// ../../testing/README.md for the list of known semantic discrepancies and
// the reasoning behind each.
package xmlcodec
