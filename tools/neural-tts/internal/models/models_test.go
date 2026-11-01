package models

import (
	"os"
	"path/filepath"
	"testing"

	ort "github.com/yalue/onnxruntime_go"
)

func assets(t *testing.T) (dir, ortDLL string) {
	t.Helper()
	dir = os.Getenv("MDVIEWER_NEURAL_DIR")
	if dir == "" {
		t.Skip("set MDVIEWER_NEURAL_DIR to resources/neural to run integration tests")
	}
	ortDLL = filepath.Join(dir, "onnxruntime.dll")
	if _, err := os.Stat(ortDLL); err != nil {
		t.Skip("onnxruntime.dll missing")
	}
	return dir, ortDLL
}

func TestParadeeSynthesizes(t *testing.T) {
	dir, ortDLL := assets(t)
	ort.SetSharedLibraryPath(ortDLL)
	if err := ort.InitializeEnvironment(); err != nil {
		t.Fatal(err)
	}
	defer ort.DestroyEnvironment()

	engine, err := New(FamilyParadee,
		filepath.Join(dir, "paradee_int8.onnx"),
		filepath.Join(dir, "paradee-config.json"),
		"")
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Destroy()

	audio, err := engine.SynthesizePhonemes("həlˈO wˈɜɹld", "af_heart", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if len(audio) == 0 {
		t.Fatal("no audio produced")
	}
}

func TestKokoroSynthesizes(t *testing.T) {
	dir, ortDLL := assets(t)
	if _, err := os.Stat(filepath.Join(dir, "kokoro-v1.0.int8.onnx")); err != nil {
		t.Skip("kokoro model missing")
	}
	ort.SetSharedLibraryPath(ortDLL)
	if err := ort.InitializeEnvironment(); err != nil {
		t.Fatal(err)
	}
	defer ort.DestroyEnvironment()

	engine, err := New(FamilyKokoro,
		filepath.Join(dir, "kokoro-v1.0.int8.onnx"),
		filepath.Join(dir, "kokoro-config.json"),
		filepath.Join(dir, "voices-v1.0.bin"))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Destroy()

	audio, err := engine.SynthesizePhonemes("həlˈO wˈɜɹld", "af_heart", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if len(audio) == 0 {
		t.Fatal("no audio produced")
	}
}
