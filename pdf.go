package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportPDF converts an already-rendered HTML document to a PDF. It asks the
// user where to save the file, then uses a Chromium-based browser (Edge or
// Chrome) in headless mode to do the actual rendering. A save dialog is used
// so the PDF always lands in a different file from the markdown source.
//
// html is the full standalone document (styles included) built by the
// frontend from the currently visible markdown.
func (a *App) ExportPDF(html, suggestedName string) (string, error) {
	if strings.TrimSpace(html) == "" {
		return "", fmt.Errorf("nothing to convert")
	}

	suggestedName = strings.TrimSpace(suggestedName)
	if suggestedName == "" {
		suggestedName = "document.pdf"
	}
	if !strings.HasSuffix(strings.ToLower(suggestedName), ".pdf") {
		suggestedName += ".pdf"
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Convert to PDF",
		DefaultFilename: suggestedName,
		Filters: []runtime.FileFilter{
			{DisplayName: "PDF Files (*.pdf)", Pattern: "*.pdf"},
		},
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // user cancelled
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".pdf") {
		path += ".pdf"
	}

	browsers, err := findPDFBrowsers()
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "mdviewer-pdf-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "document.html")
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		return "", err
	}

	// Chrome/Edge resolve relative assets against the page's own URL. The
	// rendered HTML is generated from the document, so relative paths are
	// rare, but a file:// URL keeps the page self-contained.
	pagePath := filepath.ToSlash(htmlPath)
	if !strings.HasPrefix(pagePath, "/") {
		pagePath = "/" + pagePath
	}
	pageURL := (&url.URL{Scheme: "file", Path: pagePath}).String()

	// The profile is unique per conversion so a running Edge/Chrome window
	// can't lock it and break the headless run.
	profileDir := filepath.Join(tmpDir, "profile")
	baseArgs := []string{
		"--disable-gpu",
		"--disable-extensions",
		"--disable-sync",
		"--no-first-run",
		"--no-default-browser-check",
		"--user-data-dir=" + profileDir,
		"--print-to-pdf=" + path,
	}

	// New headless first; fall back to the legacy headless mode for older
	// Edge/Chrome builds that do not understand --headless=new.
	argsSets := [][]string{
		append([]string{"--headless=new", "--no-pdf-header-footer"}, baseArgs...),
		append([]string{"--headless"}, baseArgs...),
	}

	var lastOutput string
	for _, pdfBrowser := range browsers {
		for _, args := range argsSets {
			_ = os.Remove(path)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			cmd := exec.CommandContext(ctx, pdfBrowser, append(args, pageURL)...)
			hideProcessWindow(cmd)
			out, _ := cmd.CombinedOutput()
			cancel()
			lastOutput = strings.TrimSpace(string(out))

			// The file is the source of truth: browsers sometimes return a
			// non-zero exit code even after a successful print, and the file
			// may take a moment to be fully flushed.
			for attempt := 0; attempt < 25; attempt++ {
				if validPDF(path) {
					return path, nil
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	if lastOutput != "" {
		return "", fmt.Errorf("could not create PDF: %s", lastOutput)
	}
	return "", fmt.Errorf("could not create PDF: no Chromium-based browser was able to print the document")
}

// validPDF reports whether path exists, is non-empty, and begins with the
// PDF magic bytes. The size check also avoids returning a half-written file
// when the browser is still flushing it to disk.
func validPDF(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.Size() < 5 {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	header := make([]byte, 5)
	if _, err := f.Read(header); err != nil {
		return false
	}
	return string(header) == "%PDF-"
}

// findPDFBrowsers returns every Chromium-based browser that supports
// --print-to-pdf. Edge ships with Windows 10/11, but some installations have
// headless printing disabled by policy, so Chrome is preferred when present
// and the remaining browsers act as fallbacks.
func findPDFBrowsers() ([]string, error) {
	var candidates []string
	addCandidate := func(path string) {
		if path != "" {
			candidates = append(candidates, path)
		}
	}

	if pf := os.Getenv("ProgramFiles"); pf != "" {
		addCandidate(filepath.Join(pf, "Google", "Chrome", "Application", "chrome.exe"))
		addCandidate(filepath.Join(pf, "Microsoft", "Edge", "Application", "msedge.exe"))
	}
	if pf := os.Getenv("ProgramFiles(x86)"); pf != "" {
		addCandidate(filepath.Join(pf, "Google", "Chrome", "Application", "chrome.exe"))
		addCandidate(filepath.Join(pf, "Microsoft", "Edge", "Application", "msedge.exe"))
	}
	if local := os.Getenv("LocalAppData"); local != "" {
		addCandidate(filepath.Join(local, "Google", "Chrome", "Application", "chrome.exe"))
		addCandidate(filepath.Join(local, "Microsoft", "Edge", "Application", "msedge.exe"))
	}
	addCandidate("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	addCandidate("/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge")
	addCandidate("/Applications/Chromium.app/Contents/MacOS/Chromium")

	var found []string
	seen := map[string]struct{}{}
	addFound := func(path string) {
		key := strings.ToLower(filepath.Clean(path))
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		found = append(found, path)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			addFound(candidate)
		}
	}

	for _, name := range []string{"chrome", "google-chrome", "msedge", "microsoft-edge", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			addFound(path)
		}
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("PDF conversion needs Microsoft Edge or Google Chrome, but neither was found")
	}
	return found, nil
}
