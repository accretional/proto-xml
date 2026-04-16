package xmlcodec

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html/charset"

	pb "openformat/gen/go/openformat/v1"
)

// Decode parses the given XML source into an XmlDocumentWithMetadata.
// raw_bytes on the result is always set to a copy of src.
func Decode(src []byte) (*pb.XmlDocumentWithMetadata, error) {
	raw := append([]byte(nil), src...)

	enc := detectEncoding(src)

	// Pre-transcode non-UTF-8 input (UTF-16 with BOM, windows-1252, etc.) so
	// all downstream byte-offset scanning (CDATA ranges, attribute-literal
	// extraction, text-piece reconstruction) runs on one consistent buffer.
	// raw_bytes still points to the original so UseRawBytes round-trips.
	transcoded, err := transcodeToUTF8(src, enc)
	if err != nil {
		doc := &pb.XmlDocument{WellFormed: false}
		return wrapDecoded(doc, enc, raw), fmt.Errorf("xml decode: %w", err)
	}

	doc := &pb.XmlDocument{
		WellFormed: true,
	}
	// Read the declaration from the transcoded-but-not-yet-rewritten buffer
	// so the original declared encoding lands on doc.CharacterEncodingScheme.
	if idx := bytes.Index(transcoded, []byte("<?xml")); idx >= 0 {
		if end := bytes.Index(transcoded[idx:], []byte("?>")); end > 0 {
			applyXMLDeclaration(doc, string(transcoded[idx+len("<?xml"):idx+end]))
		}
	}

	// Now rewrite the declaration to encoding="UTF-8" (if it differed) so
	// stdlib's xml decoder accepts the byte stream without calling
	// CharsetReader for mismatches.
	parsed := rewriteDeclaredEncoding(transcoded, "UTF-8")

	cdataRanges := scanCDATARanges(parsed)

	// encoding/xml only supports XML 1.0. Patch the version in the declaration
	// so that 1.1 documents decode structurally; the true declared version is
	// still applied to the proto via applyXMLDeclaration above.
	parsed = patchXMLVersion(parsed)

	dec := xml.NewDecoder(bytes.NewReader(parsed))
	dec.Strict = true
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) {
		// transcodeToUTF8 already converted the byte stream; return as-is.
		return input, nil
	}

	// Track where we are: prolog (before root), inside root, or epilog (after root).
	var (
		sawRoot  bool
		doneRoot bool
		stack    []*pb.XmlElement // element stack (current open elements)
	)

	for {
		tokOffset := dec.InputOffset()
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			doc.WellFormed = false
			return wrapDecoded(doc, enc, raw), fmt.Errorf("xml decode: %w", err)
		}

		switch t := tok.(type) {
		case xml.ProcInst:
			pi := &pb.XmlProcessingInstruction{Target: t.Target, Data: string(t.Inst)}
			if t.Target == "xml" && !sawRoot {
				// Declaration already applied from the pre-parse byte scan
				// (which runs on the transcoded-but-not-yet-rewritten buffer
				// so the original declared encoding survives).
				continue
			}
			if !sawRoot {
				doc.PrologMisc = append(doc.PrologMisc, &pb.XmlMiscNode{
					Node: &pb.XmlMiscNode_ProcessingInstruction{ProcessingInstruction: pi},
				})
			} else if doneRoot {
				doc.EpilogMisc = append(doc.EpilogMisc, &pb.XmlMiscNode{
					Node: &pb.XmlMiscNode_ProcessingInstruction{ProcessingInstruction: pi},
				})
			} else {
				appendChild(stack, &pb.XmlNode{Node: &pb.XmlNode_ProcessingInstruction{ProcessingInstruction: pi}})
			}

		case xml.Comment:
			cm := &pb.XmlComment{Content: string(t)}
			if !sawRoot {
				doc.PrologMisc = append(doc.PrologMisc, &pb.XmlMiscNode{
					Node: &pb.XmlMiscNode_Comment{Comment: cm},
				})
			} else if doneRoot {
				doc.EpilogMisc = append(doc.EpilogMisc, &pb.XmlMiscNode{
					Node: &pb.XmlMiscNode_Comment{Comment: cm},
				})
			} else {
				appendChild(stack, &pb.XmlNode{Node: &pb.XmlNode_Comment{Comment: cm}})
			}

		case xml.Directive:
			// Parse DOCTYPE as best-effort; other directives are preserved only
			// via raw_bytes. The DOCTYPE grammar is deliberately not fully
			// decoded here — see testing/README.md.
			if d := parseDoctypeDirective(string(t)); d != nil {
				doc.Doctype = d
			}

		case xml.StartElement:
			startTag := parsed[tokOffset:dec.InputOffset()]
			literals := extractAttrLiterals(startTag)
			el := startElementToProto(t, stack, literals)
			if !sawRoot {
				doc.DocumentElement = el
				sawRoot = true
			} else {
				appendChild(stack, &pb.XmlNode{Node: &pb.XmlNode_Element{Element: el}})
			}
			stack = append(stack, el)

		case xml.EndElement:
			if len(stack) == 0 {
				doc.WellFormed = false
				return wrapDecoded(doc, enc, raw), fmt.Errorf("xml decode: unexpected end element </%s>", t.Name.Local)
			}
			// Detect self-closing by checking raw bytes: an empty-element tag has
			// no explicit end element in the source. The xml package still emits
			// an EndElement token, but the start offset tokOffset points to the
			// byte AFTER the start tag — which for <foo/> is identical to the
			// start-element end offset. We approximate by scanning the raw tag.
			top := stack[len(stack)-1]
			if isSelfClosingTag(parsed, top) {
				top.SelfClosing = true
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				doneRoot = true
			}

		case xml.CharData:
			if !sawRoot || doneRoot {
				// Whitespace between prolog/epilog items is not stored as a node;
				// the raw_bytes preserves it exactly.
				continue
			}
			text := string(t)
			isCDATA := false
			endOff := dec.InputOffset()
			for _, r := range cdataRanges {
				if int64(r.start) >= tokOffset && int64(r.end) <= endOff {
					isCDATA = true
					break
				}
			}
			if isCDATA {
				appendChild(stack, &pb.XmlNode{
					Node: &pb.XmlNode_CdataSection{CdataSection: &pb.XmlCdataSection{Data: text}},
				})
			} else {
				txt := &pb.XmlText{
					Data:                     text,
					ElementContentWhitespace: isWhitespace(text),
				}
				// Reconstruct raw_pieces from the source bytes so entity /
				// character refs are preserved.
				if pieces := scanTextPieces(parsed[tokOffset:endOff]); pieces != nil {
					txt.RawPieces = pieces
				}
				appendChild(stack, &pb.XmlNode{Node: &pb.XmlNode_Text{Text: txt}})
			}
		}
	}

	if len(stack) != 0 {
		doc.WellFormed = false
		return wrapDecoded(doc, enc, raw), fmt.Errorf("xml decode: %d element(s) not closed", len(stack))
	}

	return wrapDecoded(doc, enc, raw), nil
}

func wrapDecoded(doc *pb.XmlDocument, enc *pb.XmlEncodingInfo, raw []byte) *pb.XmlDocumentWithMetadata {
	return &pb.XmlDocumentWithMetadata{
		Document:     doc,
		EncodingInfo: enc,
		RawBytes:     raw,
	}
}

func appendChild(stack []*pb.XmlElement, n *pb.XmlNode) {
	if len(stack) == 0 {
		return
	}
	parent := stack[len(stack)-1]
	parent.Children = append(parent.Children, n)
}

// startElementToProto converts an xml.StartElement into an XmlElement. Namespace
// declarations (xmlns / xmlns:prefix attributes) are routed into
// NamespaceDeclarations instead of Attributes. The literals map (keyed by
// qualified name) carries the exact literal form of each attribute value
// so that entity / character refs survive round-trip.
func startElementToProto(t xml.StartElement, stack []*pb.XmlElement, literals map[string]attrLiteral) *pb.XmlElement {
	el := &pb.XmlElement{
		NamespaceName: t.Name.Space,
		LocalName:     t.Name.Local,
	}

	inScope := map[string]string{}
	if len(stack) > 0 {
		for k, v := range stack[len(stack)-1].GetInScopeNamespaces() {
			inScope[k] = v
		}
	}

	for _, a := range t.Attr {
		// Namespace declarations
		if a.Name.Space == "xmlns" {
			el.NamespaceDeclarations = append(el.NamespaceDeclarations, &pb.XmlNamespaceDeclaration{
				Prefix: a.Name.Local, NamespaceUri: a.Value,
			})
			inScope[a.Name.Local] = a.Value
			continue
		}
		if a.Name.Space == "" && a.Name.Local == "xmlns" {
			el.NamespaceDeclarations = append(el.NamespaceDeclarations, &pb.XmlNamespaceDeclaration{
				Prefix: "", NamespaceUri: a.Value,
			})
			inScope[""] = a.Value
			continue
		}

		attr := &pb.XmlAttribute{
			NamespaceName:   a.Name.Space,
			LocalName:       a.Name.Local,
			NormalizedValue: a.Value,
			Specified:       true,
		}
		// Resolve attribute prefix. stdlib may populate Name.Space with
		// either the resolved namespace URI (if declared) or the bare prefix
		// (if undeclared). Preserve whichever form lets round-trip keep the
		// qualified name.
		if a.Name.Space != "" {
			found := false
			for p, uri := range inScope {
				if uri == a.Name.Space && p != "" {
					attr.Prefix = p
					found = true
					break
				}
			}
			if !found {
				attr.Prefix = a.Name.Space
			}
		}
		if attr.Prefix != "" {
			attr.QualifiedName = attr.Prefix + ":" + attr.LocalName
		} else {
			attr.QualifiedName = attr.LocalName
		}
		if lit, ok := literals[attr.QualifiedName]; ok {
			attr.LiteralValue = &pb.XmlAttributeValueLiteral{
				QuoteChar: lit.quote,
				Pieces:    lit.pieces,
			}
		}
		// Reserved xml:* attributes get mirrored onto the element's strong fields.
		if a.Name.Space == "http://www.w3.org/XML/1998/namespace" || (a.Name.Space == "" && strings.HasPrefix(a.Name.Local, "xml:")) {
			switch strings.TrimPrefix(a.Name.Local, "xml:") {
			case "lang":
				el.XmlLang = a.Value
			case "space":
				switch a.Value {
				case "default":
					el.XmlSpace = pb.XmlSpace_XML_SPACE_DEFAULT
				case "preserve":
					el.XmlSpace = pb.XmlSpace_XML_SPACE_PRESERVE
				}
			case "base":
				el.XmlBase = a.Value
			case "id":
				el.XmlId = a.Value
			}
		}
		el.Attributes = append(el.Attributes, attr)
	}

	// Resolve prefix/qualified_name for the element itself.
	resolved := false
	if t.Name.Space != "" {
		for prefix, uri := range inScope {
			if uri == t.Name.Space {
				el.Prefix = prefix
				resolved = true
				break
			}
		}
		// stdlib populates Name.Space with the bare prefix when no xmlns
		// declaration is found; preserve it so round-trips keep the prefix.
		if !resolved {
			el.Prefix = t.Name.Space
		}
	}
	if el.Prefix != "" {
		el.QualifiedName = el.Prefix + ":" + t.Name.Local
	} else {
		el.QualifiedName = t.Name.Local
	}
	el.InScopeNamespaces = inScope

	return el
}

func isWhitespace(s string) bool {
	for _, r := range s {
		switch r {
		case ' ', '\t', '\r', '\n':
		default:
			return false
		}
	}
	return true
}

// cdataRange records byte offsets of "<![CDATA[...]]>" sections in the raw source.
// start points to '<', end points one past '>' (so [start,end) covers the whole thing).
// For matching we care about the inner content offsets (after "<![CDATA[" up to "]]>").
type cdataRange struct{ start, end int }

func scanCDATARanges(src []byte) []cdataRange {
	var out []cdataRange
	open := []byte("<![CDATA[")
	close := []byte("]]>")
	i := 0
	for i < len(src) {
		j := bytes.Index(src[i:], open)
		if j < 0 {
			break
		}
		innerStart := i + j + len(open)
		k := bytes.Index(src[innerStart:], close)
		if k < 0 {
			break
		}
		innerEnd := innerStart + k
		out = append(out, cdataRange{start: innerStart, end: innerEnd})
		i = innerEnd + len(close)
	}
	return out
}

// isSelfClosingTag inspects the raw bytes at the element's start tag to decide
// whether it was written as <foo/> or <foo>...</foo>. We don't have exact
// offsets, so we scan near where the element would have been opened based on
// order; as a correctness fallback, we mark self_closing=true when the element
// has no children and the source contains `<localName .../>` before the
// next `<`.
func isSelfClosingTag(src []byte, el *pb.XmlElement) bool {
	if len(el.Children) != 0 {
		return false
	}
	name := el.LocalName
	if el.Prefix != "" {
		name = el.Prefix + ":" + el.LocalName
	}
	// The pattern <name( attrs)?\s*/> occurring somewhere in src is a strong
	// indicator (not perfect in the face of comments containing the same text,
	// but good enough for codec metadata).
	pat := regexp.MustCompile(`<` + regexp.QuoteMeta(name) + `(\s[^<>]*)?/>`)
	return pat.Match(src)
}

// attrLiteral is the literal representation of an attribute's value as it
// appeared in the source — delimiter quote plus the already-split pieces.
type attrLiteral struct {
	quote  pb.XmlQuoteChar
	pieces []*pb.XmlAttributeValuePiece
}

// extractAttrLiterals parses a raw start-tag slice (from the leading `<`
// through the trailing `>` or `/>`) and returns, for every attribute present,
// the literal value pieces keyed by qualified name.
//
// Hand-rolled because Go's encoding/xml already expanded the refs by the time
// we see the StartElement token.
func extractAttrLiterals(tag []byte) map[string]attrLiteral {
	out := map[string]attrLiteral{}
	i := 0
	n := len(tag)
	// Skip `<` and element name.
	if i < n && tag[i] == '<' {
		i++
	}
	for i < n && !isAttrNameBoundary(tag[i]) {
		i++
	}
	for i < n {
		// Skip whitespace.
		for i < n && isXMLSpace(tag[i]) {
			i++
		}
		if i >= n || tag[i] == '>' || tag[i] == '/' {
			break
		}
		// Read attribute name.
		nameStart := i
		for i < n && tag[i] != '=' && !isXMLSpace(tag[i]) && tag[i] != '>' {
			i++
		}
		name := string(tag[nameStart:i])
		// An empty name means we couldn't advance — bail out rather than loop.
		if nameStart == i {
			break
		}
		// Skip whitespace before `=`.
		for i < n && isXMLSpace(tag[i]) {
			i++
		}
		if i >= n || tag[i] != '=' {
			// Malformed; give up.
			break
		}
		i++ // skip `=`
		for i < n && isXMLSpace(tag[i]) {
			i++
		}
		if i >= n {
			break
		}
		if tag[i] != '"' && tag[i] != '\'' {
			// Unquoted attribute value — malformed XML; bail out.
			break
		}
		quoteChar := pb.XmlQuoteChar_XML_QUOTE_DOUBLE
		if tag[i] == '\'' {
			quoteChar = pb.XmlQuoteChar_XML_QUOTE_SINGLE
		}
		delim := tag[i]
		i++
		valueStart := i
		for i < n && tag[i] != delim {
			i++
		}
		value := tag[valueStart:i]
		if i < n {
			i++ // skip closing quote
		}
		out[name] = attrLiteral{
			quote:  quoteChar,
			pieces: splitAttrValueIntoPieces(value),
		}
	}
	return out
}

func isAttrNameBoundary(b byte) bool { return isXMLSpace(b) || b == '>' || b == '/' }

func isXMLSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

// splitAttrValueIntoPieces splits an already-unquoted literal attribute value
// into character_data / entity_ref_name / char_ref_codepoint pieces. Matches
// the shape of scanTextPieces but for attribute content.
func splitAttrValueIntoPieces(raw []byte) []*pb.XmlAttributeValuePiece {
	s := string(raw)
	var out []*pb.XmlAttributeValuePiece
	i := 0
	for i < len(s) {
		amp := strings.IndexByte(s[i:], '&')
		if amp < 0 {
			if i < len(s) {
				out = append(out, &pb.XmlAttributeValuePiece{
					Piece: &pb.XmlAttributeValuePiece_CharacterData{CharacterData: s[i:]},
				})
			}
			break
		}
		if amp > 0 {
			out = append(out, &pb.XmlAttributeValuePiece{
				Piece: &pb.XmlAttributeValuePiece_CharacterData{CharacterData: s[i : i+amp]},
			})
		}
		rest := s[i+amp:]
		semi := strings.IndexByte(rest, ';')
		if semi < 0 {
			out = append(out, &pb.XmlAttributeValuePiece{
				Piece: &pb.XmlAttributeValuePiece_CharacterData{CharacterData: rest},
			})
			break
		}
		ref := rest[1:semi] // without & and ;
		if strings.HasPrefix(ref, "#") {
			isHex := strings.HasPrefix(ref, "#x") || strings.HasPrefix(ref, "#X")
			out = append(out, &pb.XmlAttributeValuePiece{
				Piece: &pb.XmlAttributeValuePiece_CharRefCodepoint{CharRefCodepoint: parseCodepoint(ref[1:])},
			})
			if isHex {
				out = append(out, &pb.XmlAttributeValuePiece{
					Piece: &pb.XmlAttributeValuePiece_CharRefIsHex{CharRefIsHex: true},
				})
			}
		} else {
			out = append(out, &pb.XmlAttributeValuePiece{
				Piece: &pb.XmlAttributeValuePiece_EntityRefName{EntityRefName: ref},
			})
		}
		i = i + amp + semi + 1
	}
	return out
}

// scanTextPieces walks the raw bytes of a CharData token and splits it into
// character-data / entity-ref / char-ref pieces as represented in the proto.
func scanTextPieces(raw []byte) []*pb.XmlTextPiece {
	s := string(raw)
	if !strings.ContainsAny(s, "&") {
		return nil
	}
	var out []*pb.XmlTextPiece
	i := 0
	for i < len(s) {
		amp := strings.IndexByte(s[i:], '&')
		if amp < 0 {
			out = append(out, &pb.XmlTextPiece{Piece: &pb.XmlTextPiece_CharacterData{CharacterData: s[i:]}})
			break
		}
		if amp > 0 {
			out = append(out, &pb.XmlTextPiece{Piece: &pb.XmlTextPiece_CharacterData{CharacterData: s[i : i+amp]}})
		}
		rest := s[i+amp:]
		semi := strings.IndexByte(rest, ';')
		if semi < 0 {
			out = append(out, &pb.XmlTextPiece{Piece: &pb.XmlTextPiece_CharacterData{CharacterData: rest}})
			break
		}
		ref := rest[1:semi] // without & and ;
		if strings.HasPrefix(ref, "#") {
			isHex := strings.HasPrefix(ref, "#x") || strings.HasPrefix(ref, "#X")
			cp := parseCodepoint(ref[1:])
			out = append(out, &pb.XmlTextPiece{Piece: &pb.XmlTextPiece_CharRefCodepoint{CharRefCodepoint: cp}})
			if isHex {
				out = append(out, &pb.XmlTextPiece{Piece: &pb.XmlTextPiece_CharRefIsHex{CharRefIsHex: true}})
			}
		} else {
			out = append(out, &pb.XmlTextPiece{Piece: &pb.XmlTextPiece_EntityRefName{EntityRefName: ref}})
		}
		i = i + amp + semi + 1
	}
	return out
}

func parseCodepoint(s string) uint32 {
	base := 10
	if strings.HasPrefix(s, "x") || strings.HasPrefix(s, "X") {
		base = 16
		s = s[1:]
	}
	var v uint32
	for _, r := range s {
		var d uint32
		switch {
		case r >= '0' && r <= '9':
			d = uint32(r - '0')
		case r >= 'a' && r <= 'f':
			d = uint32(r-'a') + 10
		case r >= 'A' && r <= 'F':
			d = uint32(r-'A') + 10
		default:
			return v
		}
		v = v*uint32(base) + d
	}
	if v > 0x10FFFF || !utf8.ValidRune(rune(v)) {
		return 0xFFFD
	}
	return v
}

// parseDoctypeDirective extracts the name from a DOCTYPE directive body.
// The body supplied by encoding/xml excludes the leading "!" — it looks like
// `DOCTYPE root PUBLIC "id" "id"` or `DOCTYPE root [ ... ]`. We capture the
// name and external id only; the internal subset is left unparsed (see
// testing/README.md).
func parseDoctypeDirective(body string) *pb.XmlDocumentTypeDeclaration {
	fields := strings.Fields(body)
	if len(fields) < 2 || !strings.EqualFold(fields[0], "DOCTYPE") {
		return nil
	}
	d := &pb.XmlDocumentTypeDeclaration{Name: fields[1]}
	if strings.Contains(body, "[") {
		d.HasInternalSubset = true
	}
	// Capture PUBLIC/SYSTEM IDs if present.
	if idx := strings.Index(body, "PUBLIC"); idx > 0 {
		ext := &pb.XmlExternalId{}
		rest := body[idx+len("PUBLIC"):]
		parts := extractQuoted(rest, 2)
		if len(parts) >= 1 {
			ext.PublicId = parts[0]
		}
		if len(parts) >= 2 {
			ext.SystemId = parts[1]
		}
		d.ExternalId = ext
	} else if idx := strings.Index(body, "SYSTEM"); idx > 0 {
		rest := body[idx+len("SYSTEM"):]
		parts := extractQuoted(rest, 1)
		ext := &pb.XmlExternalId{}
		if len(parts) >= 1 {
			ext.SystemId = parts[0]
		}
		d.ExternalId = ext
	}
	return d
}

func extractQuoted(s string, max int) []string {
	var out []string
	for len(s) > 0 && len(out) < max {
		q := strings.IndexAny(s, `"'`)
		if q < 0 {
			break
		}
		quote := s[q]
		end := strings.IndexByte(s[q+1:], quote)
		if end < 0 {
			break
		}
		out = append(out, s[q+1:q+1+end])
		s = s[q+1+end+1:]
	}
	return out
}

// applyXMLDeclaration parses the pseudo-attrs of the <?xml ... ?> PI and writes
// them onto the XmlDocument.
func applyXMLDeclaration(doc *pb.XmlDocument, inst string) {
	attrs := parsePseudoAttrs(inst)
	switch attrs["version"] {
	case "1.0":
		doc.XmlVersion = pb.XmlVersion_XML_VERSION_1_0
	case "1.1":
		doc.XmlVersion = pb.XmlVersion_XML_VERSION_1_1
	}
	if enc, ok := attrs["encoding"]; ok {
		doc.CharacterEncodingScheme = enc
	}
	switch attrs["standalone"] {
	case "yes":
		doc.Standalone = pb.XmlStandaloneDeclaration_XML_STANDALONE_YES
	case "no":
		doc.Standalone = pb.XmlStandaloneDeclaration_XML_STANDALONE_NO
	}
}

var pseudoAttrRE = regexp.MustCompile(`(\w+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)

func parsePseudoAttrs(s string) map[string]string {
	out := map[string]string{}
	for _, m := range pseudoAttrRE.FindAllStringSubmatch(s, -1) {
		v := m[2]
		if v == "" {
			v = m[3]
		}
		out[m[1]] = v
	}
	return out
}

// patchXMLVersion rewrites `version="1.1"` → `version="1.0"` inside the XML
// declaration so stdlib's decoder accepts the stream. Byte length is
// preserved so offset tracking (used for CDATA detection) stays accurate.
func patchXMLVersion(src []byte) []byte {
	idx := bytes.Index(src, []byte("<?xml"))
	if idx < 0 {
		return src
	}
	end := bytes.Index(src[idx:], []byte("?>"))
	if end < 0 {
		return src
	}
	decl := src[idx : idx+end]
	for _, pat := range [][]byte{[]byte(`version="1.1"`), []byte(`version='1.1'`)} {
		if rel := bytes.Index(decl, pat); rel >= 0 {
			out := append([]byte(nil), src...)
			copy(out[idx+rel:], bytes.Replace(pat, []byte("1.1"), []byte("1.0"), 1))
			return out
		}
	}
	return src
}

// transcodeToUTF8 converts the source bytes to UTF-8 when a non-UTF-8
// encoding is detected (via BOM or the <?xml encoding=...?> declaration).
// Pure-ASCII / UTF-8 inputs pass through unchanged. The declaration is
// not rewritten here — the caller handles that after reading the
// declared encoding for the proto.
func transcodeToUTF8(src []byte, enc *pb.XmlEncodingInfo) ([]byte, error) {
	e, name, _ := charset.DetermineEncoding(src, "application/xml")
	if e == nil || strings.EqualFold(name, "utf-8") {
		return src, nil
	}
	out, err := e.NewDecoder().Bytes(src)
	if err != nil {
		return nil, fmt.Errorf("charset %s: %w", name, err)
	}
	// Strip UTF-8 BOM if the decoder introduced one.
	return bytes.TrimPrefix(out, []byte{0xEF, 0xBB, 0xBF}), nil
}

// rewriteDeclaredEncoding replaces the value of the encoding= pseudo-attr
// inside the XML declaration. If there's no declaration or no encoding
// attr, returns src unchanged.
func rewriteDeclaredEncoding(src []byte, target string) []byte {
	idx := bytes.Index(src, []byte("<?xml"))
	if idx < 0 {
		return src
	}
	end := bytes.Index(src[idx:], []byte("?>"))
	if end < 0 {
		return src
	}
	decl := src[idx : idx+end]
	re := regexp.MustCompile(`encoding\s*=\s*("[^"]*"|'[^']*')`)
	replaced := re.ReplaceAll(decl, []byte(`encoding="`+target+`"`))
	if bytes.Equal(replaced, decl) {
		return src
	}
	out := make([]byte, 0, len(src)+len(replaced)-len(decl))
	out = append(out, src[:idx]...)
	out = append(out, replaced...)
	out = append(out, src[idx+end:]...)
	return out
}

func detectEncoding(src []byte) *pb.XmlEncodingInfo {
	info := &pb.XmlEncodingInfo{}
	switch {
	case bytes.HasPrefix(src, []byte{0xEF, 0xBB, 0xBF}):
		info.HasBom = true
		info.BomType = pb.XmlBomType_XML_BOM_UTF_8
		info.ActualEncoding = "UTF-8"
	case bytes.HasPrefix(src, []byte{0xFE, 0xFF}):
		info.HasBom = true
		info.BomType = pb.XmlBomType_XML_BOM_UTF_16_BE
		info.ActualEncoding = "UTF-16BE"
	case bytes.HasPrefix(src, []byte{0xFF, 0xFE}):
		info.HasBom = true
		info.BomType = pb.XmlBomType_XML_BOM_UTF_16_LE
		info.ActualEncoding = "UTF-16LE"
	default:
		info.ActualEncoding = "UTF-8"
	}
	// Scan the XML declaration for a declared encoding.
	if idx := bytes.Index(src, []byte("<?xml")); idx >= 0 {
		end := bytes.Index(src[idx:], []byte("?>"))
		if end > 0 {
			attrs := parsePseudoAttrs(string(src[idx : idx+end]))
			if enc, ok := attrs["encoding"]; ok {
				info.DeclaredEncoding = enc
			}
		}
	}
	return info
}
