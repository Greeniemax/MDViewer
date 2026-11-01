// Package npz reads the subset of NumPy .npz/.npy used by Kokoro's
// voices-v1.0.bin: a zip of little-endian float32 arrays shaped (rows, 1, cols).
package npz

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type entry struct {
	data   []byte
	offset int
	rows   int
	cols   int
}

// Archive is an opened NPZ file kept in memory.
type Archive struct {
	entries map[string]*entry
}

var (
	descrRe = regexp.MustCompile(`'descr':\s*'([^']+)'`)
	shapeRe = regexp.MustCompile(`'shape':\s*\(([^)]*)\)`)
)

// Open reads every entry of the archive.
func Open(path string) (*Archive, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open npz: %w", err)
	}
	defer zr.Close()

	a := &Archive{entries: map[string]*entry{}}
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".npy") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}
		e, err := parseNPY(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		a.entries[strings.TrimSuffix(f.Name, ".npy")] = e
	}
	return a, nil
}

func parseNPY(raw []byte) (*entry, error) {
	if len(raw) < 10 || string(raw[:6]) != "\x93NUMPY" {
		return nil, fmt.Errorf("not an npy file")
	}
	major := raw[6]
	var headerLen int
	switch major {
	case 1:
		headerLen = int(binary.LittleEndian.Uint16(raw[8:10]))
	case 2, 3:
		if len(raw) < 12 {
			return nil, fmt.Errorf("truncated npy header")
		}
		headerLen = int(binary.LittleEndian.Uint32(raw[8:12]))
	default:
		return nil, fmt.Errorf("unsupported npy version %d", major)
	}
	headerStart := 10
	if major >= 2 {
		headerStart = 12
	}
	dataOffset := headerStart + headerLen
	if dataOffset > len(raw) {
		return nil, fmt.Errorf("npy header overruns file")
	}
	header := string(raw[headerStart:dataOffset])

	dm := descrRe.FindStringSubmatch(header)
	if dm == nil || dm[1] != "<f4" {
		return nil, fmt.Errorf("unsupported dtype %q", dm)
	}
	sm := shapeRe.FindStringSubmatch(header)
	if sm == nil {
		return nil, fmt.Errorf("missing shape")
	}
	var dims []int
	for _, part := range strings.Split(sm[1], ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("bad shape %q", sm[1])
		}
		dims = append(dims, n)
	}
	if len(dims) != 3 || dims[1] != 1 {
		return nil, fmt.Errorf("unsupported shape %v", dims)
	}
	return &entry{data: raw, offset: dataOffset, rows: dims[0], cols: dims[2]}, nil
}

// Names returns the entry names in sorted order.
func (a *Archive) Names() []string {
	names := make([]string, 0, len(a.entries))
	for n := range a.entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Shape returns the number of style rows and the vector width.
func (a *Archive) Shape(name string) (rows, cols int, err error) {
	e, ok := a.entries[name]
	if !ok {
		return 0, 0, fmt.Errorf("voice %q not found", name)
	}
	return e.rows, e.cols, nil
}

// Row returns one style vector.
func (a *Archive) Row(name string, row int) ([]float32, error) {
	e, ok := a.entries[name]
	if !ok {
		return nil, fmt.Errorf("voice %q not found", name)
	}
	if row < 0 || row >= e.rows {
		return nil, fmt.Errorf("row %d out of range [0,%d)", row, e.rows)
	}
	start := e.offset + row*e.cols*4
	end := start + e.cols*4
	if end > len(e.data) {
		return nil, fmt.Errorf("row %d overruns data", row)
	}
	buf := bytes.NewReader(e.data[start:end])
	out := make([]float32, e.cols)
	if err := binary.Read(buf, binary.LittleEndian, out); err != nil {
		return nil, err
	}
	return out, nil
}
