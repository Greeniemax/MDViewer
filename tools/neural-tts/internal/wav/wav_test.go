package wav

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteHeaderAndClip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.wav")
	if err := Write(path, []float32{0, 1.5, -1.5, 0.5}, 24000); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		t.Fatalf("bad RIFF header: %q %q", raw[0:4], raw[8:12])
	}
	if got := binary.LittleEndian.Uint16(raw[22:24]); got != 1 {
		t.Fatalf("channels = %d", got)
	}
	if got := binary.LittleEndian.Uint32(raw[24:28]); got != 24000 {
		t.Fatalf("rate = %d", got)
	}
	if got := binary.LittleEndian.Uint16(raw[34:36]); got != 16 {
		t.Fatalf("bits = %d", got)
	}
	if got := binary.LittleEndian.Uint32(raw[40:44]); got != 8 {
		t.Fatalf("data size = %d", got)
	}
	samples := []int16{0, 32767, -32767, 16383}
	for i, want := range samples {
		got := int16(binary.LittleEndian.Uint16(raw[44+i*2 : 46+i*2]))
		if got != want {
			t.Fatalf("sample %d = %d, want %d", i, got, want)
		}
	}
}
