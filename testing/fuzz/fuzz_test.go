// Package fuzz runs fuzz tests against the xmlcodec Decode/Encode pair.
//
// Primary goals:
//  1. Decode must never panic on arbitrary input.
//  2. For inputs that Decode accepts, a structural re-encode followed by
//     another Decode must also not panic.
//
// Seeded with every file under data/ so the fuzzer starts from interesting
// real-world shapes rather than random bytes.
package fuzz

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"openformat/xmlcodec"
)

func seedCorpus(f *testing.F) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		f.Fatalf("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "data")
	matches, err := filepath.Glob(filepath.Join(root, "*", "*.xml"))
	if err != nil {
		f.Fatalf("glob: %v", err)
	}
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			f.Fatalf("read %s: %v", m, err)
		}
		f.Add(b)
	}
	// A few structural primitives that exercise edge cases.
	f.Add([]byte(`<?xml version="1.0"?><r/>`))
	f.Add([]byte(`<r><![CDATA[]]></r>`))
	f.Add([]byte(`<r>&amp;</r>`))
	f.Add([]byte(``))
	f.Add([]byte(`<r`))      // truncated
	f.Add([]byte(`<r></x>`)) // mismatched tags
}

// FuzzDecode checks that arbitrary inputs never cause Decode to panic.
func FuzzDecode(f *testing.F) {
	seedCorpus(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Decode panic on %q: %v", data, r)
			}
		}()
		_, _ = xmlcodec.Decode(data)
	})
}

// FuzzRoundTrip decodes, re-encodes, and re-decodes. On inputs Decode
// accepts, neither of those steps may panic, and the structural re-encode
// must itself be decodable.
func FuzzRoundTrip(f *testing.F) {
	seedCorpus(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on %q: %v", data, r)
			}
		}()
		md, err := xmlcodec.Decode(data)
		if err != nil {
			return
		}
		out, err := xmlcodec.EncodeWithOptions(md.Document, xmlcodec.EncodeOptions{})
		if err != nil {
			return
		}
		if _, err := xmlcodec.Decode(out); err != nil {
			// A structural re-encode that's not parseable is a real bug —
			// unless the original was not well-formed. Require well-formed
			// inputs to survive round-trip.
			if md.Document.WellFormed {
				t.Errorf("structural round-trip failed:\n  input : %q\n  output: %q\n  err   : %v", data, out, err)
			}
		}
	})
}
