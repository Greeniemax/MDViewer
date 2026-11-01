package tts

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLengthScale(t *testing.T) {
	cases := []struct {
		rate int
		want float64
	}{
		{175, 1.0}, // natural pace
		{350, 0.5}, // clamped fast
		{50, 2.0},  // clamped slow (175/50=3.5 -> 2.0)
		{0, 1.0},   // guard against divide-by-zero
		{-10, 1.0}, // guard against negative
	}
	for _, c := range cases {
		if got := LengthScale(c.rate); got != c.want {
			t.Errorf("LengthScale(%d) = %v, want %v", c.rate, got, c.want)
		}
	}
	// 250 WPM -> 0.7 (within range), allow float tolerance.
	if got := LengthScale(250); got < 0.69 || got > 0.71 {
		t.Errorf("LengthScale(250) = %v, want ~0.7", got)
	}
}

func TestPiperVoiceID(t *testing.T) {
	if id, ok := PiperVoiceID("piper\ten_US-amy-medium"); !ok || id != "en_US-amy-medium" {
		t.Errorf("got (%q,%v), want (en_US-amy-medium,true)", id, ok)
	}
	if _, ok := PiperVoiceID("sapi\tMicrosoft David"); ok {
		t.Errorf("sapi selector should not be a piper voice")
	}
	if _, ok := PiperVoiceID("Alex"); ok {
		t.Errorf("bare macOS voice should not be a piper voice")
	}
}

func TestBinaryName(t *testing.T) {
	if BinaryName("windows") != "piper.exe" {
		t.Errorf("windows binary name wrong")
	}
	if BinaryName("darwin") != "piper" {
		t.Errorf("darwin binary name wrong")
	}
}

func TestCandidateDirs(t *testing.T) {
	win := CandidateDirs("C:/app", "windows")
	if len(win) != 1 || win[0] != filepath.Join("C:/app", "piper") {
		t.Errorf("windows candidates = %v", win)
	}
	mac := CandidateDirs("/A/Contents/MacOS", "darwin")
	// expect both <exeDir>/piper and the .app Resources/piper sibling
	wantRes := filepath.Join("/A/Contents", "Resources", "piper")
	found := false
	for _, d := range mac {
		if d == wantRes {
			found = true
		}
	}
	if !found {
		t.Errorf("darwin candidates missing %q: got %v", wantRes, mac)
	}
}

func TestVoiceLabel(t *testing.T) {
	good := []byte(`{"dataset":"amy","language":{"code":"en_US"}}`)
	if got := VoiceLabel(good, "en_US-amy-medium"); got != "Amy (Natural) — en_US" {
		t.Errorf("VoiceLabel good = %q", got)
	}
	if got := VoiceLabel([]byte("not json"), "en_US-ryan-medium"); !strings.Contains(got, "Natural") {
		t.Errorf("VoiceLabel fallback should contain 'Natural', got %q", got)
	}
}

func TestNeuralVoiceID(t *testing.T) {
	family, id, ok := NeuralVoiceID("paradee\taf_heart")
	if !ok || family != "paradee" || id != "af_heart" {
		t.Fatalf("got %q %q %v", family, id, ok)
	}
	family, id, ok = NeuralVoiceID("kokoro\tbf_isabella")
	if !ok || family != "kokoro" || id != "bf_isabella" {
		t.Fatalf("got %q %q %v", family, id, ok)
	}
	if _, _, ok := NeuralVoiceID("piper\ten_US-amy-medium"); ok {
		t.Fatal("piper selector must not match")
	}
	if _, _, ok := NeuralVoiceID("default"); ok {
		t.Fatal("plain selector must not match")
	}
}

func TestNeuralSpeed(t *testing.T) {
	if got := NeuralSpeed(175); got != 1.0 {
		t.Fatalf("175 WPM = %v, want 1.0", got)
	}
	if got := NeuralSpeed(250); got < 1.4 || got > 1.5 {
		t.Fatalf("250 WPM = %v", got)
	}
	if got := NeuralSpeed(50); got != 0.5 {
		t.Fatalf("clamp low = %v", got)
	}
	if got := NeuralSpeed(500); got != 2.0 {
		t.Fatalf("clamp high = %v", got)
	}
}
