//go:build windows

package espeak

import (
	"os"
	"strings"
	"testing"
)

func TestPhonemizeHello(t *testing.T) {
	const dir = "../../../../resources/piper/engine/windows/piper"
	dll := dir + "/espeak-ng.dll"
	data := dir + "/espeak-ng-data"
	if _, err := os.Stat(dll); err != nil {
		t.Skip("espeak-ng.dll not fetched; run scripts/fetch-piper.ps1")
	}
	if err := Init(dll, data); err != nil {
		t.Fatal(err)
	}
	got, err := Phonemize("Hello world.", "en-us")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "həl") || !strings.Contains(got, "wˈ") {
		t.Fatalf("unexpected phonemes: %q", got)
	}
}
