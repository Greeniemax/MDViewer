package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidPDF(t *testing.T) {
	dir := t.TempDir()

	good := filepath.Join(dir, "good.pdf")
	if err := os.WriteFile(good, []byte("%PDF-1.7\nfake body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !validPDF(good) {
		t.Fatal("validPDF rejected a file with a PDF header")
	}

	bad := filepath.Join(dir, "bad.pdf")
	if err := os.WriteFile(bad, []byte("<html>not a pdf</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if validPDF(bad) {
		t.Fatal("validPDF accepted a non-PDF file")
	}

	if validPDF(filepath.Join(dir, "missing.pdf")) {
		t.Fatal("validPDF accepted a missing file")
	}
}

// TestPrintHTMLToPDF exercises the real headless-browser pipeline. It is
// skipped on machines without Edge/Chrome so `go test ./...` stays portable.
func TestPrintHTMLToPDF(t *testing.T) {
	if _, err := findPDFBrowsers(); err != nil {
		t.Skipf("no Chromium-based browser available: %v", err)
	}

	dir := t.TempDir()
	out := filepath.Join(dir, "sample.pdf")
	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>MDViewer PDF test</title>
<style>
    @page { size: A4; margin: 18mm; }
    body { font-family: sans-serif; font-size: 12pt; }
</style>
</head>
<body>
<h1>MDViewer PDF test</h1>
<p>Hello <strong>world</strong>. This page verifies that the headless browser
pipeline writes a valid PDF document.</p>
<ul><li>one</li><li>two</li></ul>
</body>
</html>`

	if err := printHTMLToPDF(html, out); err != nil {
		t.Fatalf("printHTMLToPDF: %v", err)
	}

	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() < 500 {
		t.Fatalf("PDF is suspiciously small: %d bytes", info.Size())
	}
	if !validPDF(out) {
		t.Fatal("output does not start with the PDF header")
	}
}
