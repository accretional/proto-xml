package xmlcodec

import (
	"strings"
	"testing"

	pb "openformat/gen/go/openformat/v1"
)

func TestDecodeBasic(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<greeting>Hello, world!</greeting>`)
	md, err := Decode(src)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if md.Document.XmlVersion != pb.XmlVersion_XML_VERSION_1_0 {
		t.Errorf("xml version = %v, want 1.0", md.Document.XmlVersion)
	}
	if md.Document.CharacterEncodingScheme != "UTF-8" {
		t.Errorf("encoding = %q, want UTF-8", md.Document.CharacterEncodingScheme)
	}
	root := md.Document.DocumentElement
	if root == nil || root.LocalName != "greeting" {
		t.Fatalf("root element = %+v, want <greeting>", root)
	}
	if len(root.Children) != 1 {
		t.Fatalf("root.Children = %d, want 1", len(root.Children))
	}
	txt := root.Children[0].GetText()
	if txt == nil || txt.Data != "Hello, world!" {
		t.Errorf("text = %+v, want %q", txt, "Hello, world!")
	}
	if string(md.RawBytes) != string(src) {
		t.Errorf("raw_bytes not preserved")
	}
}

func TestDecodeAttributesAndNamespaces(t *testing.T) {
	src := []byte(`<root xmlns="urn:a" xmlns:x="urn:x" x:id="42" plain="yes"><x:child/></root>`)
	md, err := Decode(src)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	root := md.Document.DocumentElement
	if root.NamespaceName != "urn:a" {
		t.Errorf("root namespace = %q, want urn:a", root.NamespaceName)
	}
	if len(root.NamespaceDeclarations) != 2 {
		t.Errorf("ns decls = %d, want 2", len(root.NamespaceDeclarations))
	}
	if len(root.Attributes) != 2 {
		t.Errorf("attrs = %d, want 2", len(root.Attributes))
	}
	child := root.Children[0].GetElement()
	if child == nil {
		t.Fatalf("no child element")
	}
	if child.NamespaceName != "urn:x" {
		t.Errorf("child ns = %q, want urn:x", child.NamespaceName)
	}
	if !child.SelfClosing {
		t.Errorf("child should be self-closing")
	}
}

func TestDecodeCDATAAndComments(t *testing.T) {
	src := []byte(`<?xml version="1.0"?>
<!-- prolog comment -->
<r><![CDATA[<raw> & stuff]]><!-- inner --><?php echo 1; ?></r>`)
	md, err := Decode(src)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(md.Document.PrologMisc) != 1 {
		t.Fatalf("prolog misc count = %d, want 1", len(md.Document.PrologMisc))
	}
	if md.Document.PrologMisc[0].GetComment() == nil {
		t.Errorf("prolog misc[0] should be comment")
	}
	root := md.Document.DocumentElement
	if len(root.Children) != 3 {
		t.Fatalf("root children = %d, want 3", len(root.Children))
	}
	if cd := root.Children[0].GetCdataSection(); cd == nil || cd.Data != "<raw> & stuff" {
		t.Errorf("cdata = %+v", cd)
	}
	if root.Children[1].GetComment() == nil {
		t.Errorf("expected comment child")
	}
	if pi := root.Children[2].GetProcessingInstruction(); pi == nil || pi.Target != "php" {
		t.Errorf("pi = %+v", pi)
	}
}

func TestDecodeEntityAndCharRefs(t *testing.T) {
	src := []byte(`<r>&amp; &#65; &#x4E;</r>`)
	md, err := Decode(src)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	root := md.Document.DocumentElement
	// encoding/xml merges into a single CharData after entity resolution.
	if len(root.Children) != 1 {
		t.Fatalf("children = %d", len(root.Children))
	}
	txt := root.Children[0].GetText()
	if txt == nil {
		t.Fatalf("no text")
	}
	if txt.Data != "& A N" {
		t.Errorf("resolved text = %q, want %q", txt.Data, "& A N")
	}
}

func TestDecodeXMLReservedAttrs(t *testing.T) {
	src := []byte(`<r xml:lang="en" xml:space="preserve" xml:base="http://ex/" xml:id="r1"/>`)
	md, err := Decode(src)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	el := md.Document.DocumentElement
	if el.XmlLang != "en" {
		t.Errorf("xml:lang = %q", el.XmlLang)
	}
	if el.XmlSpace != pb.XmlSpace_XML_SPACE_PRESERVE {
		t.Errorf("xml:space = %v", el.XmlSpace)
	}
	if el.XmlBase != "http://ex/" {
		t.Errorf("xml:base = %q", el.XmlBase)
	}
	if el.XmlId != "r1" {
		t.Errorf("xml:id = %q", el.XmlId)
	}
}

func TestEncodeBasic(t *testing.T) {
	doc := &pb.XmlDocument{
		XmlVersion:              pb.XmlVersion_XML_VERSION_1_0,
		CharacterEncodingScheme: "UTF-8",
		DocumentElement: &pb.XmlElement{
			LocalName:     "greeting",
			QualifiedName: "greeting",
			Children: []*pb.XmlNode{
				{Node: &pb.XmlNode_Text{Text: &pb.XmlText{Data: "Hello, world!"}}},
			},
		},
	}
	out, err := Encode(doc)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<greeting>Hello, world!</greeting>`
	if string(out) != want {
		t.Errorf("got %q\nwant %q", out, want)
	}
}

func TestEncodeEscapes(t *testing.T) {
	doc := &pb.XmlDocument{
		XmlVersion: pb.XmlVersion_XML_VERSION_1_0,
		DocumentElement: &pb.XmlElement{
			LocalName:     "r",
			QualifiedName: "r",
			Attributes: []*pb.XmlAttribute{
				{LocalName: "a", NormalizedValue: `<&">`, Specified: true},
			},
			Children: []*pb.XmlNode{
				{Node: &pb.XmlNode_Text{Text: &pb.XmlText{Data: "<tag> & stuff"}}},
			},
		},
	}
	out, err := EncodeWithOptions(doc, EncodeOptions{OmitXMLDeclaration: true})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := `<r a="&lt;&amp;&quot;>">&lt;tag&gt; &amp; stuff</r>`
	if string(out) != want {
		t.Errorf("got %q\nwant %q", out, want)
	}
}

func TestRoundTripViaRawBytes(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<root><child a="1"/><![CDATA[raw<>]]></root>`)
	md, err := Decode(src)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	out, err := EncodeMetadata(md, EncodeOptions{UseRawBytes: true})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(out) != string(src) {
		t.Errorf("raw round-trip mismatch\ngot:  %q\nwant: %q", out, src)
	}
}

func TestRoundTripStructural(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8"?><root xmlns="urn:a"><c>hi</c></root>`)
	md, err := Decode(src)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	out, err := EncodeWithOptions(md.Document, EncodeOptions{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	md2, err := Decode(out)
	if err != nil {
		t.Fatalf("re-Decode: %v", err)
	}
	// Compare root locally.
	if md2.Document.DocumentElement.LocalName != "root" {
		t.Errorf("lost root name")
	}
	if !strings.Contains(string(out), `xmlns="urn:a"`) {
		t.Errorf("lost default namespace in output: %s", out)
	}
}

func TestAttrLiteralRoundTrip(t *testing.T) {
	// Structural re-encode must preserve entity/char refs inside attribute
	// values byte-for-byte when the literal form was captured at decode.
	src := []byte(`<r a="amp=&amp; lt=&lt; apos=&apos;" b='hex=&#x4E; dec=&#65;' c="plain"/>`)
	md, err := Decode(src)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	out, err := EncodeWithOptions(md.Document, EncodeOptions{OmitXMLDeclaration: true})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(out) != string(src) {
		t.Errorf("attr literal round-trip mismatch\ngot:  %s\nwant: %s", out, src)
	}
	// Sanity: a.LiteralValue should exist with multiple pieces.
	el := md.Document.DocumentElement
	if len(el.Attributes) == 0 || el.Attributes[0].LiteralValue == nil {
		t.Fatalf("attr literal value not captured")
	}
	if len(el.Attributes[0].LiteralValue.Pieces) < 3 {
		t.Errorf("expected multiple literal pieces, got %d", len(el.Attributes[0].LiteralValue.Pieces))
	}
}

func TestDecodeBOM(t *testing.T) {
	src := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`<r/>`)...)
	md, err := Decode(src)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !md.EncodingInfo.HasBom {
		t.Errorf("expected BOM to be detected")
	}
	if md.EncodingInfo.BomType != pb.XmlBomType_XML_BOM_UTF_8 {
		t.Errorf("bom type = %v", md.EncodingInfo.BomType)
	}
}
