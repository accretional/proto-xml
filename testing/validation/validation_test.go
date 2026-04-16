// Package validation runs a single suite of checks across every XML fixture
// under data/. For the moment this is one parametrized test — TestValidate —
// driven as table-tests over the discovered files.
package validation

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	pb "openformat/gen/go/openformat/v1"
	"openformat/internal/xmlcodec"
)

// dataDir locates the repo-level data/ directory relative to this test file,
// so tests are runnable via `go test ./...` from any working directory.
func dataDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// testing/validation/validation_test.go → repo root is two dirs up.
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "data")
}

// discoverXMLFiles returns every *.xml file under data/.
func discoverXMLFiles(t *testing.T) []string {
	t.Helper()
	root := dataDir(t)
	matches, err := filepath.Glob(filepath.Join(root, "*", "*.xml"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no xml fixtures found under %s", root)
	}
	return matches
}

// TestValidate is the single suite: for every fixture, Decode must succeed,
// the result must carry raw_bytes identical to the file, encoder must produce
// a byte-identical output when asked to use raw_bytes, and a structural
// re-encode must be parseable by the decoder (round-trip-via-structure).
func TestValidate(t *testing.T) {
	for _, path := range discoverXMLFiles(t) {
		path := path
		name := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			src := readFile(t, path)

			md, err := xmlcodec.Decode(src)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if md.Document == nil {
				t.Fatal("nil Document")
			}
			if md.Document.DocumentElement == nil {
				t.Fatal("no document element produced")
			}
			if string(md.RawBytes) != string(src) {
				t.Fatalf("raw_bytes differs from source")
			}

			// Raw-bytes round-trip must be byte-identical.
			out, err := xmlcodec.EncodeMetadata(md, xmlcodec.EncodeOptions{UseRawBytes: true})
			if err != nil {
				t.Fatalf("EncodeMetadata(raw): %v", err)
			}
			if string(out) != string(src) {
				t.Fatalf("raw round-trip mismatch")
			}

			// Structural encode must be re-decodable.
			structured, err := xmlcodec.EncodeWithOptions(md.Document, xmlcodec.EncodeOptions{})
			if err != nil {
				t.Fatalf("Encode(structural): %v", err)
			}
			md2, err := xmlcodec.Decode(structured)
			if err != nil {
				t.Fatalf("re-decode structural: %v\n--- output was ---\n%s", err, structured)
			}
			if got := md2.Document.DocumentElement.LocalName; got != md.Document.DocumentElement.LocalName {
				t.Errorf("root local name after round trip: got %q, want %q", got, md.Document.DocumentElement.LocalName)
			}

			// Version must round-trip.
			if md.Document.XmlVersion != pb.XmlVersion_XML_VERSION_UNSPECIFIED &&
				md.Document.XmlVersion != md2.Document.XmlVersion &&
				!strings.Contains(path, "p06_utf8_bom") {
				t.Errorf("xml version round trip: %v -> %v", md.Document.XmlVersion, md2.Document.XmlVersion)
			}
		})
	}
}
