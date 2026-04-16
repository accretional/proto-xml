// Command gen-fixtures creates programmatic XML test fixtures under
// data/programmatic/ by building XmlDocument protos and encoding them with the
// xmlcodec package. Idempotent: skips files that already exist.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	pb "openformat/gen/go/openformat/v1"
	"openformat/xmlcodec"
)

func main() {
	outDir := flag.String("out", "data/programmatic", "output directory")
	force := flag.Bool("force", false, "overwrite existing files")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		die("mkdir %s: %v", *outDir, err)
	}

	fixtures := []struct {
		name string
		doc  *pb.XmlDocument
	}{
		{"p01_simple.xml", simpleDoc()},
		{"p02_namespaced.xml", namespacedDoc()},
		{"p03_rss.xml", rssDoc()},
		{"p04_nested.xml", nestedDoc(6)},
		{"p05_mixed_content.xml", mixedContentDoc()},
	}

	for _, f := range fixtures {
		path := filepath.Join(*outDir, f.name)
		if !*force {
			if _, err := os.Stat(path); err == nil {
				fmt.Printf("skip %s (exists)\n", path)
				continue
			}
		}
		out, err := xmlcodec.Encode(f.doc)
		if err != nil {
			die("encode %s: %v", f.name, err)
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			die("write %s: %v", path, err)
		}
		fmt.Printf("wrote %s (%d bytes)\n", path, len(out))
	}

	// BOM-prefixed fixture built from raw bytes, since the encoder doesn't
	// emit a BOM itself.
	bomPath := filepath.Join(*outDir, "p06_utf8_bom.xml")
	if *force || !exists(bomPath) {
		body := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<root>bom-prefixed</root>\n")
		bom := []byte{0xEF, 0xBB, 0xBF}
		if err := os.WriteFile(bomPath, append(bom, body...), 0o644); err != nil {
			die("write %s: %v", bomPath, err)
		}
		fmt.Printf("wrote %s (with UTF-8 BOM)\n", bomPath)
	} else {
		fmt.Printf("skip %s (exists)\n", bomPath)
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen-fixtures: "+format+"\n", args...)
	os.Exit(1)
}

func text(s string) *pb.XmlNode {
	return &pb.XmlNode{Node: &pb.XmlNode_Text{Text: &pb.XmlText{Data: s}}}
}

func child(el *pb.XmlElement) *pb.XmlNode {
	return &pb.XmlNode{Node: &pb.XmlNode_Element{Element: el}}
}

func simpleDoc() *pb.XmlDocument {
	return &pb.XmlDocument{
		XmlVersion:              pb.XmlVersion_XML_VERSION_1_0,
		CharacterEncodingScheme: "UTF-8",
		DocumentElement: &pb.XmlElement{
			LocalName: "greeting", QualifiedName: "greeting",
			Children: []*pb.XmlNode{text("Hello from the proto encoder!")},
		},
	}
}

func namespacedDoc() *pb.XmlDocument {
	root := &pb.XmlElement{
		LocalName: "catalog", QualifiedName: "catalog",
		NamespaceDeclarations: []*pb.XmlNamespaceDeclaration{
			{Prefix: "", NamespaceUri: "urn:example:catalog"},
			{Prefix: "dc", NamespaceUri: "http://purl.org/dc/elements/1.1/"},
		},
		NamespaceName: "urn:example:catalog",
	}
	for _, title := range []string{"Gödel, Escher, Bach", "Structure and Interpretation of Computer Programs"} {
		item := &pb.XmlElement{
			LocalName: "book", QualifiedName: "book", NamespaceName: "urn:example:catalog",
			Children: []*pb.XmlNode{
				child(&pb.XmlElement{
					LocalName: "title", QualifiedName: "dc:title", Prefix: "dc",
					NamespaceName: "http://purl.org/dc/elements/1.1/",
					Children:      []*pb.XmlNode{text(title)},
				}),
			},
		}
		root.Children = append(root.Children, child(item))
	}
	return &pb.XmlDocument{
		XmlVersion:              pb.XmlVersion_XML_VERSION_1_0,
		CharacterEncodingScheme: "UTF-8",
		DocumentElement:         root,
	}
}

func rssDoc() *pb.XmlDocument {
	channel := &pb.XmlElement{
		LocalName: "channel", QualifiedName: "channel",
		Children: []*pb.XmlNode{
			child(elText("title", "proto-xml programmatic feed")),
			child(elText("link", "https://example.com/proto-feed")),
			child(elText("description", "Generated directly from an XmlDocument proto.")),
			child(&pb.XmlElement{
				LocalName: "item", QualifiedName: "item",
				Children: []*pb.XmlNode{
					child(elText("title", "Proto-generated post")),
					child(elText("link", "https://example.com/proto-feed/1")),
					child(&pb.XmlElement{
						LocalName: "description", QualifiedName: "description",
						Children: []*pb.XmlNode{
							{Node: &pb.XmlNode_CdataSection{CdataSection: &pb.XmlCdataSection{
								Data: "<p>Body with <strong>inline</strong> HTML.</p>",
							}}},
						},
					}),
				},
			}),
		},
	}
	root := &pb.XmlElement{
		LocalName: "rss", QualifiedName: "rss",
		Attributes: []*pb.XmlAttribute{
			{LocalName: "version", NormalizedValue: "2.0", Specified: true},
		},
		Children: []*pb.XmlNode{child(channel)},
	}
	return &pb.XmlDocument{
		XmlVersion:              pb.XmlVersion_XML_VERSION_1_0,
		CharacterEncodingScheme: "UTF-8",
		DocumentElement:         root,
	}
}

func nestedDoc(depth int) *pb.XmlDocument {
	leaf := &pb.XmlElement{
		LocalName: "leaf", QualifiedName: "leaf",
		Children: []*pb.XmlNode{text(fmt.Sprintf("depth=%d", depth))},
	}
	var cur *pb.XmlElement = leaf
	for i := depth - 1; i >= 1; i-- {
		cur = &pb.XmlElement{
			LocalName: fmt.Sprintf("n%d", i), QualifiedName: fmt.Sprintf("n%d", i),
			Children: []*pb.XmlNode{child(cur)},
		}
	}
	return &pb.XmlDocument{
		XmlVersion:              pb.XmlVersion_XML_VERSION_1_0,
		CharacterEncodingScheme: "UTF-8",
		DocumentElement:         cur,
	}
}

func mixedContentDoc() *pb.XmlDocument {
	p := &pb.XmlElement{
		LocalName: "p", QualifiedName: "p",
		Children: []*pb.XmlNode{
			text("The "),
			child(elText("em", "quick")),
			text(" brown "),
			child(elText("strong", "fox")),
			text(" jumps over the lazy dog."),
		},
	}
	return &pb.XmlDocument{
		XmlVersion:              pb.XmlVersion_XML_VERSION_1_0,
		CharacterEncodingScheme: "UTF-8",
		DocumentElement: &pb.XmlElement{
			LocalName: "doc", QualifiedName: "doc",
			Children: []*pb.XmlNode{child(p)},
		},
	}
}

func elText(name, body string) *pb.XmlElement {
	return &pb.XmlElement{
		LocalName: name, QualifiedName: name,
		Children: []*pb.XmlNode{text(body)},
	}
}
