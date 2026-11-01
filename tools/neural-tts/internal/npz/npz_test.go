package npz

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// npyFloat32 builds a minimal v1.0 .npy payload with the given shape.
func npyFloat32(shape []int, vals []float32) []byte {
	shapeStr := "("
	for i, d := range shape {
		if i > 0 {
			shapeStr += ", "
		}
		shapeStr += fmt.Sprint(d)
	}
	if len(shape) == 1 {
		shapeStr += ","
	}
	shapeStr += ")"
	header := fmt.Sprintf("{'descr': '<f4', 'fortran_order': False, 'shape': %s, }", shapeStr)
	// v1.0 pads the header so data starts on a 16-byte boundary.
	pad := 16 - ((10 + len(header) + 1) % 16)
	header += string(bytes.Repeat([]byte{' '}, pad)) + "\n"

	var buf bytes.Buffer
	buf.WriteString("\x93NUMPY")
	buf.WriteByte(1)
	buf.WriteByte(0)
	binary.Write(&buf, binary.LittleEndian, uint16(len(header)))
	buf.WriteString(header)
	binary.Write(&buf, binary.LittleEndian, vals)
	return buf.Bytes()
}

func writeNPZ(t *testing.T, entries map[string][]float32, shape []int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "voices.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, vals := range entries {
		w, err := zw.Create(name + ".npy")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(npyFloat32(shape, vals)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenAndRow(t *testing.T) {
	vals := make([]float32, 3*1*4)
	for i := range vals {
		vals[i] = float32(i)
	}
	path := writeNPZ(t, map[string][]float32{"af_heart": vals}, []int{3, 1, 4})

	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	names := a.Names()
	if len(names) != 1 || names[0] != "af_heart" {
		t.Fatalf("Names = %v", names)
	}

	rows, cols, err := a.Shape("af_heart")
	if err != nil {
		t.Fatal(err)
	}
	if rows != 3 || cols != 4 {
		t.Fatalf("Shape = %d,%d", rows, cols)
	}

	row, err := a.Row("af_heart", 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []float32{4, 5, 6, 7}
	for i := range want {
		if row[i] != want[i] {
			t.Fatalf("Row = %v, want %v", row, want)
		}
	}
}

func TestRowOutOfRange(t *testing.T) {
	path := writeNPZ(t, map[string][]float32{"af_heart": make([]float32, 8)}, []int{2, 1, 4})
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Row("af_heart", 2); err == nil {
		t.Fatal("expected an error for row 2")
	}
}
