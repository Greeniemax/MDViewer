//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNeuralParadeeRendersViaServer exercises the app-side helper client against
// the bundled helper. It skips unless the neural assets have been built into
// build/bin/neural (or MDVIEWER_NEURAL_DIR points at them).
func TestNeuralParadeeRendersViaServer(t *testing.T) {
	dir := os.Getenv("MDVIEWER_NEURAL_DIR")
	if dir == "" {
		dir = filepath.Join("build", "bin", "neural")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(abs, "neural-tts.exe")); err != nil {
		t.Skip("neural helper not built; run build.bat")
	}
	t.Setenv("MDVIEWER_NEURAL_DIR", abs)

	a := NewApp()
	defer a.stopNeuralServers()

	// Two calls prove the server survives between lines.
	for _, text := range []string{"Hello world.", "This is the second line."} {
		path, err := a.synthesizeNeuralFile("paradee", "af_heart", text, 175, nil)
		if err != nil {
			t.Fatalf("synthesize %q: %v", text, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("%q produced no audio: %v", text, err)
		}
		os.Remove(path)
	}
}
