// Package benchmarks runs Decode/Encode benchmarks across every fixture
// under data/. Use `go test -bench=. ./testing/benchmarks/...` (the test
// scripts wrap this).
package benchmarks

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"openformat/internal/xmlcodec"
)

func fixtures(tb testing.TB) []fixture {
	tb.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatalf("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "data")
	matches, err := filepath.Glob(filepath.Join(root, "*", "*.xml"))
	if err != nil {
		tb.Fatalf("glob: %v", err)
	}
	out := make([]fixture, 0, len(matches))
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			tb.Fatalf("read %s: %v", m, err)
		}
		name := filepath.Base(filepath.Dir(m)) + "/" + filepath.Base(m)
		out = append(out, fixture{name: name, data: b})
	}
	return out
}

type fixture struct {
	name string
	data []byte
}

func BenchmarkDecode(b *testing.B) {
	for _, f := range fixtures(b) {
		f := f
		b.Run(f.name, func(b *testing.B) {
			b.SetBytes(int64(len(f.data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := xmlcodec.Decode(f.data); err != nil {
					b.Fatalf("decode: %v", err)
				}
			}
		})
	}
}

func BenchmarkEncodeStructural(b *testing.B) {
	for _, f := range fixtures(b) {
		md, err := xmlcodec.Decode(f.data)
		if err != nil {
			continue
		}
		f := f
		doc := md.Document
		b.Run(f.name, func(b *testing.B) {
			b.SetBytes(int64(len(f.data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := xmlcodec.Encode(doc); err != nil {
					b.Fatalf("encode: %v", err)
				}
			}
		})
	}
}

func BenchmarkEncodeRawBytes(b *testing.B) {
	for _, f := range fixtures(b) {
		md, err := xmlcodec.Decode(f.data)
		if err != nil {
			continue
		}
		f := f
		b.Run(f.name, func(b *testing.B) {
			b.SetBytes(int64(len(f.data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := xmlcodec.EncodeMetadata(md, xmlcodec.EncodeOptions{UseRawBytes: true}); err != nil {
					b.Fatalf("encode: %v", err)
				}
			}
		})
	}
}

func BenchmarkRoundTrip(b *testing.B) {
	for _, f := range fixtures(b) {
		f := f
		b.Run(f.name, func(b *testing.B) {
			b.SetBytes(int64(len(f.data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				md, err := xmlcodec.Decode(f.data)
				if err != nil {
					b.Fatalf("decode: %v", err)
				}
				if _, err := xmlcodec.Encode(md.Document); err != nil {
					b.Fatalf("encode: %v", err)
				}
			}
		})
	}
}
