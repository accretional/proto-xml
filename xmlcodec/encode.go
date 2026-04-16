package xmlcodec

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	pb "openformat/gen/go/openformat/v1"
)

// EncodeOptions tune Encode behavior.
type EncodeOptions struct {
	// UseRawBytes returns XmlDocumentWithMetadata.RawBytes verbatim when set.
	// Only applicable to EncodeMetadata.
	UseRawBytes bool
	// OmitXMLDeclaration skips writing the <?xml ... ?> prolog.
	OmitXMLDeclaration bool
}

// Encode writes an XmlDocument as XML bytes.
func Encode(doc *pb.XmlDocument) ([]byte, error) {
	return encodeDocument(doc, EncodeOptions{})
}

// EncodeWithOptions writes an XmlDocument as XML bytes using the given options.
func EncodeWithOptions(doc *pb.XmlDocument, opts EncodeOptions) ([]byte, error) {
	return encodeDocument(doc, opts)
}

// EncodeMetadata writes an XmlDocumentWithMetadata; honours UseRawBytes.
func EncodeMetadata(md *pb.XmlDocumentWithMetadata, opts EncodeOptions) ([]byte, error) {
	if md == nil {
		return nil, fmt.Errorf("encode: nil metadata")
	}
	if opts.UseRawBytes && len(md.RawBytes) > 0 {
		return append([]byte(nil), md.RawBytes...), nil
	}
	return encodeDocument(md.Document, opts)
}

func encodeDocument(doc *pb.XmlDocument, opts EncodeOptions) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("encode: nil document")
	}
	var buf bytes.Buffer

	if !opts.OmitXMLDeclaration {
		writeXMLDeclaration(&buf, doc)
	}

	for _, misc := range doc.PrologMisc {
		writeMiscNode(&buf, misc)
	}

	if doc.Doctype != nil {
		writeDoctype(&buf, doc.Doctype)
	}

	if doc.DocumentElement != nil {
		if err := writeElement(&buf, doc.DocumentElement); err != nil {
			return nil, err
		}
	}

	for _, misc := range doc.EpilogMisc {
		writeMiscNode(&buf, misc)
	}

	return buf.Bytes(), nil
}

func writeXMLDeclaration(buf *bytes.Buffer, doc *pb.XmlDocument) {
	ver := "1.0"
	if doc.XmlVersion == pb.XmlVersion_XML_VERSION_1_1 {
		ver = "1.1"
	}
	enc := doc.CharacterEncodingScheme
	if enc == "" {
		enc = "UTF-8"
	}
	buf.WriteString(`<?xml version="`)
	buf.WriteString(ver)
	buf.WriteString(`" encoding="`)
	buf.WriteString(enc)
	buf.WriteString(`"`)
	switch doc.Standalone {
	case pb.XmlStandaloneDeclaration_XML_STANDALONE_YES:
		buf.WriteString(` standalone="yes"`)
	case pb.XmlStandaloneDeclaration_XML_STANDALONE_NO:
		buf.WriteString(` standalone="no"`)
	}
	buf.WriteString("?>\n")
}

func writeDoctype(buf *bytes.Buffer, d *pb.XmlDocumentTypeDeclaration) {
	buf.WriteString("<!DOCTYPE ")
	buf.WriteString(d.Name)
	if ext := d.ExternalId; ext != nil {
		if ext.PublicId != "" {
			fmt.Fprintf(buf, ` PUBLIC "%s" "%s"`, ext.PublicId, ext.SystemId)
		} else if ext.SystemId != "" {
			fmt.Fprintf(buf, ` SYSTEM "%s"`, ext.SystemId)
		}
	}
	// Internal subset is not re-emitted (see testing/README.md). Leaving it
	// out is safe when the doctype is purely declarative.
	buf.WriteString(">\n")
}

func writeMiscNode(buf *bytes.Buffer, m *pb.XmlMiscNode) {
	if pi := m.GetProcessingInstruction(); pi != nil {
		writeProcessingInstruction(buf, pi)
		buf.WriteByte('\n')
	}
	if cm := m.GetComment(); cm != nil {
		writeComment(buf, cm)
		buf.WriteByte('\n')
	}
}

func writeProcessingInstruction(buf *bytes.Buffer, pi *pb.XmlProcessingInstruction) {
	buf.WriteString("<?")
	buf.WriteString(pi.Target)
	if pi.Data != "" {
		buf.WriteByte(' ')
		buf.WriteString(pi.Data)
	}
	buf.WriteString("?>")
}

func writeComment(buf *bytes.Buffer, c *pb.XmlComment) {
	buf.WriteString("<!--")
	buf.WriteString(c.Content)
	buf.WriteString("-->")
}

func writeElement(buf *bytes.Buffer, el *pb.XmlElement) error {
	if el == nil {
		return nil
	}
	qname := qualifiedName(el)
	buf.WriteByte('<')
	buf.WriteString(qname)

	for _, nsd := range el.NamespaceDeclarations {
		if nsd.Prefix == "" {
			fmt.Fprintf(buf, ` xmlns="%s"`, escapeAttrValue(nsd.NamespaceUri))
		} else {
			fmt.Fprintf(buf, ` xmlns:%s="%s"`, nsd.Prefix, escapeAttrValue(nsd.NamespaceUri))
		}
	}
	writeReservedXMLAttrs(buf, el)
	for _, a := range el.Attributes {
		if isReservedXMLAttr(a) {
			// already emitted via writeReservedXMLAttrs when set on el
			continue
		}
		name := a.QualifiedName
		if name == "" {
			name = a.LocalName
			if a.Prefix != "" {
				name = a.Prefix + ":" + a.LocalName
			} else if a.NamespaceName != "" {
				// Resolve prefix from element's in-scope namespaces.
				for p, uri := range el.GetInScopeNamespaces() {
					if uri == a.NamespaceName && p != "" {
						name = p + ":" + a.LocalName
						break
					}
				}
			}
		}
		writeAttribute(buf, name, a)
	}

	if len(el.Children) == 0 && el.SelfClosing {
		buf.WriteString("/>")
		return nil
	}
	buf.WriteByte('>')
	for _, child := range el.Children {
		if err := writeNode(buf, child); err != nil {
			return err
		}
	}
	buf.WriteString("</")
	buf.WriteString(qname)
	buf.WriteByte('>')
	return nil
}

func qualifiedName(el *pb.XmlElement) string {
	if el.QualifiedName != "" {
		return el.QualifiedName
	}
	if el.Prefix != "" {
		return el.Prefix + ":" + el.LocalName
	}
	return el.LocalName
}

func writeNode(buf *bytes.Buffer, n *pb.XmlNode) error {
	switch v := n.Node.(type) {
	case *pb.XmlNode_Element:
		return writeElement(buf, v.Element)
	case *pb.XmlNode_Text:
		if len(v.Text.RawPieces) > 0 {
			for _, p := range v.Text.RawPieces {
				writeTextPiece(buf, p)
			}
			return nil
		}
		buf.WriteString(escapeCharData(v.Text.Data))
	case *pb.XmlNode_CdataSection:
		buf.WriteString("<![CDATA[")
		buf.WriteString(v.CdataSection.Data)
		buf.WriteString("]]>")
	case *pb.XmlNode_Comment:
		writeComment(buf, v.Comment)
	case *pb.XmlNode_ProcessingInstruction:
		writeProcessingInstruction(buf, v.ProcessingInstruction)
	case *pb.XmlNode_EntityReference:
		buf.WriteByte('&')
		buf.WriteString(v.EntityReference.Name)
		buf.WriteByte(';')
	case *pb.XmlNode_CharacterReference:
		if v.CharacterReference.IsHex {
			fmt.Fprintf(buf, "&#x%X;", v.CharacterReference.Codepoint)
		} else {
			fmt.Fprintf(buf, "&#%d;", v.CharacterReference.Codepoint)
		}
	}
	return nil
}

func writeTextPiece(buf *bytes.Buffer, p *pb.XmlTextPiece) {
	switch v := p.Piece.(type) {
	case *pb.XmlTextPiece_CharacterData:
		buf.WriteString(escapeCharData(v.CharacterData))
	case *pb.XmlTextPiece_EntityRefName:
		buf.WriteByte('&')
		buf.WriteString(v.EntityRefName)
		buf.WriteByte(';')
	case *pb.XmlTextPiece_CharRefCodepoint:
		buf.WriteString("&#")
		buf.WriteString(strconv.FormatUint(uint64(v.CharRefCodepoint), 10))
		buf.WriteByte(';')
	case *pb.XmlTextPiece_CharRefIsHex:
		// hex marker for the preceding codepoint — already emitted as decimal;
		// callers that want hex encoding should emit the codepoint themselves.
	}
}

// writeAttribute writes ` name=<quote>value<quote>` using the literal form
// when XmlAttribute.LiteralValue is populated (preserving the original entity
// and character references), falling back to the escaped normalized value.
func writeAttribute(buf *bytes.Buffer, name string, a *pb.XmlAttribute) {
	buf.WriteByte(' ')
	buf.WriteString(name)
	buf.WriteByte('=')
	if lit := a.LiteralValue; lit != nil && len(lit.Pieces) > 0 {
		quote := byte('"')
		if lit.QuoteChar == pb.XmlQuoteChar_XML_QUOTE_SINGLE {
			quote = '\''
		}
		buf.WriteByte(quote)
		writeAttrLiteralPieces(buf, lit.Pieces)
		buf.WriteByte(quote)
		return
	}
	buf.WriteByte('"')
	buf.WriteString(escapeAttrValue(a.NormalizedValue))
	buf.WriteByte('"')
}

func writeAttrLiteralPieces(buf *bytes.Buffer, pieces []*pb.XmlAttributeValuePiece) {
	for i := 0; i < len(pieces); i++ {
		p := pieces[i]
		switch v := p.Piece.(type) {
		case *pb.XmlAttributeValuePiece_CharacterData:
			buf.WriteString(v.CharacterData)
		case *pb.XmlAttributeValuePiece_EntityRefName:
			buf.WriteByte('&')
			buf.WriteString(v.EntityRefName)
			buf.WriteByte(';')
		case *pb.XmlAttributeValuePiece_CharRefCodepoint:
			// Peek ahead: a trailing char_ref_is_hex=true marker means the
			// source used hex form. Emit accordingly and consume the marker.
			hex := false
			if i+1 < len(pieces) {
				if _, ok := pieces[i+1].Piece.(*pb.XmlAttributeValuePiece_CharRefIsHex); ok {
					hex = true
					i++
				}
			}
			if hex {
				fmt.Fprintf(buf, "&#x%X;", v.CharRefCodepoint)
			} else {
				fmt.Fprintf(buf, "&#%d;", v.CharRefCodepoint)
			}
		case *pb.XmlAttributeValuePiece_CharRefIsHex:
			// Orphan hex marker (no preceding codepoint) — skip.
		}
	}
}

func writeReservedXMLAttrs(buf *bytes.Buffer, el *pb.XmlElement) {
	if el.XmlLang != "" {
		fmt.Fprintf(buf, ` xml:lang="%s"`, escapeAttrValue(el.XmlLang))
	}
	switch el.XmlSpace {
	case pb.XmlSpace_XML_SPACE_DEFAULT:
		buf.WriteString(` xml:space="default"`)
	case pb.XmlSpace_XML_SPACE_PRESERVE:
		buf.WriteString(` xml:space="preserve"`)
	}
	if el.XmlBase != "" {
		fmt.Fprintf(buf, ` xml:base="%s"`, escapeAttrValue(el.XmlBase))
	}
	if el.XmlId != "" {
		fmt.Fprintf(buf, ` xml:id="%s"`, escapeAttrValue(el.XmlId))
	}
}

func isReservedXMLAttr(a *pb.XmlAttribute) bool {
	if a.Prefix == "xml" {
		switch a.LocalName {
		case "lang", "space", "base", "id":
			return true
		}
	}
	if a.NamespaceName == "http://www.w3.org/XML/1998/namespace" {
		return true
	}
	return false
}

// escapeCharData replaces the five XML metacharacters that are illegal (or
// risky) in element text content: &, <, > — and, defensively, the two quote
// chars. Quotes aren't strictly required in text nodes, but normalizing to
// entities is safer and matches the output of most XML libraries.
func escapeCharData(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}

func escapeAttrValue(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		`"`, "&quot;",
		"\n", "&#10;",
		"\r", "&#13;",
		"\t", "&#9;",
	)
	return replacer.Replace(s)
}
