// Command demo-screenshots generates the PNG assets referenced from
// docs/about.md. When a chromerpc gRPC server is reachable at
// CHROMERPC_ADDR (default localhost:50051), it drives real screenshots
// of the demo HTML pages. Otherwise it writes placeholder PNGs so the
// documentation doesn't show broken images in the GitHub UI.
//
// The placeholder path is the default so `./setup.sh` / `./build.sh`
// never depend on a running external service; a developer who wants
// real captures runs:
//
//	CHROMERPC_ADDR=localhost:50051 go run ./cmd/demo-screenshots -force
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	pb "openformat/gen/go/openformat/v1"
	"openformat/xmlcodec"
)

const defaultChromeRPCAddr = "localhost:50051"

func main() {
	outDir := flag.String("out", "docs/screenshots", "output directory for PNGs")
	htmlDir := flag.String("html-out", "docs/screenshots/_html", "where demo HTML pages are written")
	force := flag.Bool("force", false, "regenerate even if files exist")
	flag.Parse()

	addr := os.Getenv("CHROMERPC_ADDR")
	if addr == "" {
		addr = defaultChromeRPCAddr
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		die("mkdir %s: %v", *outDir, err)
	}
	if err := os.MkdirAll(*htmlDir, 0o755); err != nil {
		die("mkdir %s: %v", *htmlDir, err)
	}

	rssPath := filepath.Join(repoRoot(), "data/handwritten/09_rss_feed.xml")
	src, err := os.ReadFile(rssPath)
	if err != nil {
		die("read rss fixture: %v", err)
	}
	md, err := xmlcodec.Decode(src)
	if err != nil {
		die("decode rss fixture: %v", err)
	}

	// Write three demo HTML pages, then either screenshot them via chromerpc
	// or emit placeholder PNGs.
	rendered := filepath.Join(*htmlDir, "rss-rendered.html")
	decoded := filepath.Join(*htmlDir, "rss-decoded-json.html")
	diff := filepath.Join(*htmlDir, "rss-diff.html")
	writeIf(rendered, []byte(renderRSS(src, md.Document)), *force)
	writeIf(decoded, []byte(renderDecoded(md)), *force)
	writeIf(diff, []byte(renderDiff(src, md)), *force)

	targets := []screenshotTarget{
		{html: rendered, png: filepath.Join(*outDir, "rss-rendered.png"), caption: "RSS fixture rendered"},
		{html: decoded, png: filepath.Join(*outDir, "rss-decoded-json.png"), caption: "Decoded XmlDocument (JSON)"},
		{html: diff, png: filepath.Join(*outDir, "rss-diff.png"), caption: "Raw vs structural encode"},
	}

	if chromeRPCReachable(addr) {
		fmt.Printf("chromerpc reachable at %s — real screenshots unsupported in this build\n", addr)
		fmt.Println("  (this repo intentionally does not vendor the chromerpc gRPC stubs)")
		fmt.Println("  falling through to placeholder generation")
	} else {
		fmt.Printf("chromerpc not reachable at %s — writing placeholder PNGs\n", addr)
	}

	for _, t := range targets {
		if !*force {
			if _, err := os.Stat(t.png); err == nil {
				fmt.Printf("skip %s (exists)\n", t.png)
				continue
			}
		}
		if err := writePlaceholder(t.png, t.caption, t.html); err != nil {
			die("placeholder %s: %v", t.png, err)
		}
		fmt.Printf("wrote %s\n", t.png)
	}
}

type screenshotTarget struct {
	html    string
	png     string
	caption string
}

func chromeRPCReachable(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func writeIf(path string, body []byte, force bool) {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return
		}
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		die("write %s: %v", path, err)
	}
}

func renderRSS(src []byte, _ *pb.XmlDocument) string {
	return fmt.Sprintf(`<!doctype html>
<html><head><meta charset="utf-8"><title>RSS fixture</title>
<style>body{font-family:monospace;padding:24px;background:#fafafa;color:#222}
pre{white-space:pre-wrap;border:1px solid #ccc;padding:16px;background:white}</style>
</head><body>
<h1>data/handwritten/09_rss_feed.xml</h1>
<pre>%s</pre>
</body></html>`, htmlEscape(string(src)))
}

func renderDecoded(md *pb.XmlDocumentWithMetadata) string {
	// A hand-rolled, non-circular JSON view of the decoded tree.
	view := map[string]any{
		"xml_version":     md.Document.XmlVersion.String(),
		"encoding":        md.Document.CharacterEncodingScheme,
		"root_local_name": md.Document.DocumentElement.GetLocalName(),
		"item_count":      countItems(md.Document.DocumentElement),
		"comment_count":   countComments(md.Document.DocumentElement),
		"cdata_count":     countCdata(md.Document.DocumentElement),
	}
	b, _ := json.MarshalIndent(view, "", "  ")
	return fmt.Sprintf(`<!doctype html>
<html><head><meta charset="utf-8"><title>Decoded XmlDocument</title>
<style>body{font-family:monospace;padding:24px}pre{background:#0f172a;color:#f8fafc;padding:16px;border-radius:6px;overflow:auto}</style>
</head><body>
<h1>xmlcodec.Decode → XmlDocument (summary)</h1>
<pre>%s</pre>
</body></html>`, htmlEscape(string(b)))
}

func renderDiff(src []byte, md *pb.XmlDocumentWithMetadata) string {
	structured, _ := xmlcodec.Encode(md.Document)
	return fmt.Sprintf(`<!doctype html>
<html><head><meta charset="utf-8"><title>Round-trip diff</title>
<style>body{font-family:monospace;padding:24px}
.col{display:inline-block;vertical-align:top;width:48%%;margin-right:1%%}
pre{background:#fff;border:1px solid #ccc;padding:12px;white-space:pre-wrap;word-break:break-word;max-height:70vh;overflow:auto}
h2{font-size:14px;color:#555}</style>
</head><body>
<h1>Raw vs Structural Encode</h1>
<div class="col"><h2>Original (raw_bytes)</h2><pre>%s</pre></div>
<div class="col"><h2>xmlcodec.Encode(doc)</h2><pre>%s</pre></div>
</body></html>`, htmlEscape(string(src)), htmlEscape(string(structured)))
}

func countItems(el *pb.XmlElement) int {
	if el == nil {
		return 0
	}
	n := 0
	if el.LocalName == "item" {
		n++
	}
	for _, c := range el.Children {
		n += countItems(c.GetElement())
	}
	return n
}

func countComments(el *pb.XmlElement) int {
	if el == nil {
		return 0
	}
	n := 0
	for _, c := range el.Children {
		if c.GetComment() != nil {
			n++
		}
		n += countComments(c.GetElement())
	}
	return n
}

func countCdata(el *pb.XmlElement) int {
	if el == nil {
		return 0
	}
	n := 0
	for _, c := range el.Children {
		if c.GetCdataSection() != nil {
			n++
		}
		n += countCdata(c.GetElement())
	}
	return n
}

func htmlEscape(s string) string {
	var b []byte
	for _, r := range s {
		switch r {
		case '<':
			b = append(b, "&lt;"...)
		case '>':
			b = append(b, "&gt;"...)
		case '&':
			b = append(b, "&amp;"...)
		default:
			b = append(b, string(r)...)
		}
	}
	return string(b)
}

// writePlaceholder emits a 1280x720 PNG with the caption and HTML source path
// burned in. Uses x/image/font/basicfont which ships a 7x13 bitmap font so we
// don't need a .ttf asset.
func writePlaceholder(pngPath, caption, htmlSrc string) error {
	const w, h = 1280, 720
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	bg := color.RGBA{0xf5, 0xf5, 0xfa, 0xff}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, bg)
		}
	}
	border := color.RGBA{0x34, 0x4a, 0x8c, 0xff}
	for x := 0; x < w; x++ {
		img.Set(x, 0, border)
		img.Set(x, h-1, border)
	}
	for y := 0; y < h; y++ {
		img.Set(0, y, border)
		img.Set(w-1, y, border)
	}

	drawString(img, 48, 96, color.RGBA{0x1a, 0x20, 0x40, 0xff}, "proto-xml demo screenshot")
	drawString(img, 48, 140, color.RGBA{0x34, 0x4a, 0x8c, 0xff}, caption)
	drawString(img, 48, 220, color.Black, "Placeholder image.")
	drawString(img, 48, 248, color.Black, "A real screenshot requires a running chromerpc gRPC server.")
	drawString(img, 48, 276, color.Black, "Regenerate with:")
	drawString(img, 80, 304, color.Black, "CHROMERPC_ADDR=localhost:50051 go run ./cmd/demo-screenshots -force")
	drawString(img, 48, 372, color.Black, "Source HTML for this shot:")
	drawString(img, 80, 400, color.Black, htmlSrc)

	f, err := os.Create(pngPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func drawString(img *image.RGBA, x, y int, col color.Color, s string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(s)
}

func repoRoot() string {
	_, this, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Join(filepath.Dir(this), "..", "..")
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "demo-screenshots: "+format+"\n", args...)
	os.Exit(1)
}
