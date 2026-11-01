# Paradee + Kokoro Neural TTS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Paradee (4 voices) and Kokoro English (28 voices) as optional TTS voices in the Windows MDViewer app, using a native `neural-tts` helper behind the existing prefetch/file-then-play pipeline.

**Architecture:** A separate Go module at `tools/neural-tts` converts text → espeak-ng IPA → misaki phonemes → ONNX Runtime → WAV. The Wails app shells out to it once per line, exactly like Piper, and merges its voices into the existing dropdown.

**Tech Stack:** Go 1.23, cgo, `github.com/yalue/onnxruntime_go` v1.14.0, ONNX Runtime 1.20.0, espeak-ng 1.52 DLL, Paradee 1.2 int8 ONNX, Kokoro-82M v1.0 int8 ONNX.

**Spec:** `Docs/superpowers/specs/2026-11-01-paradee-kokoro-tts-design.md`

---

## Prerequisites

Run these once before Task 1 (they are also automated in Task 14):

```bash
# from repo root
powershell -ExecutionPolicy Bypass -File scripts/fetch-piper.ps1   # provides espeak-ng.dll + data
gcc --version                                                       # MinGW required for cgo
go version                                                          # 1.23+
```

The neural assets are downloaded in Task 9 into `resources/neural/`:

| File | SHA-256 |
|---|---|
| `paradee_int8.onnx` | `a6989796692ac173597e15d405c971989202d5c73cfad65f22e90bd7b6c97da1` |
| `kokoro-v1.0.int8.onnx` | `6e742170d309016e5891a994e1ce1559c702a2ccd0075e67ef7157974f6406cb` |
| `voices-v1.0.bin` | `bca610b8308e8d99f32e6fe4197e7ec01679264efed0cac9140fe9c29f1fbf7d` |
| `onnxruntime-win-x64-1.20.0.zip` | `b372de85cedd9387a0d4386b982265e8420e5bcc2f29394317e76525b832942e` |

Committed config files (already small, created in Task 9):
`resources/neural/paradee-config.json`, `resources/neural/kokoro-config.json`.

---

## File Map

```
tools/neural-tts/                     NEW separate Go module
├── go.mod, go.sum
├── main.go                           CLI, flags, list-voices, orchestrates
├── internal/espeak/espeak_windows.go runtime DLL loading + phonemization
├── internal/espeak/espeak_other.go   stub for non-Windows
├── internal/misaki/misaki.go         eSpeak→misaki port
├── internal/misaki/misaki_test.go
├── internal/phonemes/phonemes.go     clause split, ids, chunking
├── internal/phonemes/phonemes_test.go
├── internal/npz/npz.go               NPY/NPZ reader for Kokoro style rows
├── internal/npz/npz_test.go
├── internal/wav/wav.go               24 kHz mono 16-bit PCM writer
├── internal/wav/wav_test.go
├── internal/models/models.go         ORT session + family adapters
└── internal/models/models_test.go    integration (skips without assets)

internal/tts/tts.go                   MODIFY: add selector + speed helpers
internal/tts/tts_test.go              MODIFY: add tests
tts_neural.go                         NEW app integration
tts_pipeline.go                       MODIFY: dispatch
app.go                                MODIFY: voices + Stop sweep
frontend/src/main.js                  MODIFY: dropdown groups
scripts/fetch-neural.ps1              NEW
build.bat                             MODIFY: helper build + bundle
.gitignore                            MODIFY: ignore neural binaries
Docs/NEURAL_TTS.md                    NEW
README.md, Project-Tracker.md         MODIFY
```

---

## Task 1: Scaffold the neural-tts module

**Files:**
- Create: `tools/neural-tts/go.mod`
- Create: `tools/neural-tts/main.go`

- [ ] **Step 1: Create the module**

```bash
cd tools/neural-tts
go mod init github.com/Greeniemax/MDViewer/tools/neural-tts
```

`go.mod` should contain:

```go
module github.com/Greeniemax/MDViewer/tools/neural-tts

go 1.23
```

- [ ] **Step 2: Write the CLI stub**

`tools/neural-tts/main.go`:

```go
// Command neural-tts renders one line of text to a 24 kHz mono WAV using
// Paradee or Kokoro-82M. It is invoked per line by MDViewer and communicates
// through flags + stdin, mirroring the Piper helper.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		family     string
		modelPath  string
		configPath string
		voicesPath string
		voice      string
		speed      float64
		lang       string
		ortPath    string
		espeakDLL  string
		espeakData string
		outPath    string
		listVoices bool
	)
	flag.StringVar(&family, "family", "paradee", "paradee|kokoro")
	flag.StringVar(&modelPath, "model", "", "path to the ONNX model")
	flag.StringVar(&configPath, "config", "", "path to config.json")
	flag.StringVar(&voicesPath, "voices", "", "Kokoro voices-v1.0.bin")
	flag.StringVar(&voice, "voice", "", "voice name")
	flag.Float64Var(&speed, "speed", 1.0, "1.0 = normal")
	flag.StringVar(&lang, "lang", "en-us", "en-us|en-gb")
	flag.StringVar(&ortPath, "ort", "", "path to onnxruntime.dll")
	flag.StringVar(&espeakDLL, "espeak-dll", "", "path to espeak-ng.dll")
	flag.StringVar(&espeakData, "espeak-data", "", "path to espeak-ng-data")
	flag.StringVar(&outPath, "out", "", "output WAV path")
	flag.BoolVar(&listVoices, "list-voices", false, "print voices and exit")
	flag.Parse()

	fmt.Fprintln(os.Stderr, "neural-tts: not implemented yet")
	os.Exit(1)
}
```

- [ ] **Step 3: Build**

Run: `cd tools/neural-tts && go build ./...`
Expected: builds with no output.

- [ ] **Step 4: Commit**

```bash
git add tools/neural-tts
git commit -m "chore(neural-tts): scaffold helper module"
```

---

## Task 2: Port the eSpeak→misaki mapping

**Files:**
- Create: `tools/neural-tts/internal/misaki/misaki_test.go`
- Create: `tools/neural-tts/internal/misaki/misaki.go`

- [ ] **Step 1: Write the failing tests**

`tools/neural-tts/internal/misaki/misaki_test.go`:

```go
package misaki

import "testing"

func TestAmericanDiphthongsAndRhotics(t *testing.T) {
	got := ToMisaki("həlˈoʊ wˈɜːld", false)
	want := "həlˈO wˈɜɹld"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestBritishEaBeforeE(t *testing.T) {
	got := ToMisaki("ðeə", true)
	want := "ðɛː"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestSyllabicConsonant(t *testing.T) {
	got := ToMisaki("bˈʌt n̩", false)
	want := "bˈʌt ᵊn"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestWordFinalSyllabicL(t *testing.T) {
	got := ToMisaki("nˈeɪvəl", false)
	want := "nˈAvᵊl"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestAmericanFlapAndGlottal(t *testing.T) {
	got := ToMisaki("wˈɔɾɚ ʔ", false)
	want := "wˈɔTəɹ t"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestBritishTheBeforeVowel(t *testing.T) {
	got := ToMisaki("ðɪ æpl", true)
	want := "ði æpl"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestArticleAKeepsOpenA(t *testing.T) {
	if got, want := ToMisaki("ɐ æpl", false), "ɐ æpl"; got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
	if got, want := ToMisaki("ɐbˈʌv", false), "əbˈʌv"; got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd tools/neural-tts && go test ./internal/misaki -v`
Expected: FAIL — `undefined: ToMisaki`.

- [ ] **Step 3: Implement the mapping**

`tools/neural-tts/internal/misaki/misaki.go`:

```go
// Package misaki converts eSpeak NG IPA into the phoneme alphabet Kokoro and
// Paradee were trained on.
//
// The rules are a Go port of web/misaki.js from
// github.com/sahilmahendrakar/paradee (Apache-2.0), itself derived from
// misaki/espeak.py (Apache-2.0). Keep the replacement order: it matters.
package misaki

import (
	"strings"
	"unicode"
)

const boundary = " \t\r\n;:,.!?—…\"“”()"

var replacements = []struct{ from, to string }{
	{"ʔˌn̩", "ʔn"}, {"ʔn̩", "ʔn"},
	{"aɪ", "I"}, {"aʊ", "W"},
	{"dʒ", "ʤ"},
	{"eɪ", "A"}, {"e", "A"},
	{"tʃ", "ʧ"},
	{"ɔɪ", "Y"},
	{"ʲo", "jo"}, {"ʲə", "jə"}, {"ʲ", ""},
	{"ɚ", "əɹ"},
	{"r", "ɹ"},
	{"x", "k"}, {"ç", "k"},
	{"ɬ", "l"},
	{"̃", ""},
}

// ToMisaki normalizes one eSpeak phoneme string. british selects the rules for
// the bf_/bm_ voices.
func ToMisaki(ps string, british bool) string {
	if british {
		ps = strings.ReplaceAll(ps, "eə", "ɛː")
	}
	for _, r := range replacements {
		ps = strings.ReplaceAll(ps, r.from, r.to)
	}
	ps = syllabicConsonants(ps)
	ps = finalSyllabicL(ps)
	ps = replaceOpenA(ps, british)

	if british {
		ps = strings.ReplaceAll(ps, "iə", "ɪə")
		ps = strings.ReplaceAll(ps, "əʊ", "Q")
		ps = theBeforeVowel(ps)
	} else {
		ps = strings.ReplaceAll(ps, "oʊ", "O")
		ps = strings.ReplaceAll(ps, "ɜːɹ", "ɜɹ")
		ps = strings.ReplaceAll(ps, "ɜː", "ɜɹ")
		ps = strings.ReplaceAll(ps, "ɪə", "iə")
		ps = strings.ReplaceAll(ps, "ː", "")
	}
	ps = strings.ReplaceAll(ps, "o", "ɔ")
	ps = strings.ReplaceAll(ps, "ɾ", "T")
	return strings.ReplaceAll(ps, "ʔ", "t")
}

func isBoundary(r rune) bool { return strings.ContainsRune(boundary, r) }

// syllabicConsonants turns "n̩" into "ᵊn" (the misaki spelling of a syllabic
// consonant) and drops any leftover combining vertical line.
func syllabicConsonants(ps string) string {
	rs := []rune(ps)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if rs[i] == '̩' {
			continue
		}
		if i+1 < len(rs) && rs[i+1] == '̩' && !unicode.IsSpace(rs[i]) {
			b.WriteRune('ᵊ')
			b.WriteRune(rs[i])
			i++
			continue
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}

// finalSyllabicL rewrites word-final "əl" as "ᵊl" (naval → nˈAvᵊl).
func finalSyllabicL(ps string) string {
	rs := []rune(ps)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if i+1 < len(rs) && rs[i] == 'ə' && rs[i+1] == 'l' {
			if i+2 >= len(rs) || isBoundary(rs[i+2]) {
				b.WriteString("ᵊl")
				i++
				continue
			}
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}

// replaceOpenA keeps misaki's ɐ for the article "a" (and British "an").
func replaceOpenA(ps string, british bool) string {
	rs := []rune(ps)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if rs[i] != 'ɐ' {
			b.WriteRune(rs[i])
			continue
		}
		j := i + 1
		if british && j < len(rs) && rs[j] == 'n' {
			j++
		}
		if j >= len(rs) || isBoundary(rs[j]) {
			b.WriteRune('ɐ')
		} else {
			b.WriteRune('ə')
		}
	}
	return b.String()
}

// theBeforeVowel turns "ðɪ" into "ði" for the word "the" before a vowel.
func theBeforeVowel(ps string) string {
	rs := []rune(ps)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if i+1 < len(rs) && rs[i] == 'ð' && rs[i+1] == 'ɪ' {
			beforeOK := i == 0 || unicode.IsSpace(rs[i-1])
			after := rune(0)
			if i+2 < len(rs) {
				after = rs[i+2]
			}
			afterOK := after == 0 || unicode.IsSpace(after)
			if beforeOK && afterOK {
				b.WriteString("ði")
				i++
				continue
			}
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}
```

- [ ] **Step 4: Run the tests**

Run: `cd tools/neural-tts && go test ./internal/misaki -v`
Expected: all 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/neural-tts/internal/misaki
git commit -m "feat(neural-tts): port the espeak-to-misaki mapping"
```

---

## Task 3: Clause splitting, token IDs, and chunking

**Files:**
- Create: `tools/neural-tts/internal/phonemes/phonemes_test.go`
- Create: `tools/neural-tts/internal/phonemes/phonemes.go`

- [ ] **Step 1: Write the failing tests**

`tools/neural-tts/internal/phonemes/phonemes_test.go`:

```go
package phonemes

import (
	"reflect"
	"testing"
)

func TestSplitClauses(t *testing.T) {
	got := Split("Hello, world. Is it faster? Yes!")
	want := []Clause{
		{Text: "Hello", Punct: ','},
		{Text: "world", Punct: '.'},
		{Text: "Is it faster", Punct: '?'},
		{Text: "Yes", Punct: '!'},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %#v, want %#v", got, want)
	}
}

func TestSplitSkipsEmptyClauses(t *testing.T) {
	got := Split("Wait... what")
	want := []Clause{
		{Text: "Wait", Punct: '.'},
		{Text: "what", Punct: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %#v, want %#v", got, want)
	}
}

func TestIDsPadsAndDropsUnknown(t *testing.T) {
	vocab := map[string]int64{"a": 43, "b": 44, " ": 16}
	got := IDs("a?b", vocab, 510)
	want := []int64{0, 43, 44, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestIDsCapsAtMax(t *testing.T) {
	vocab := map[string]int64{"a": 43}
	got := IDs("aaaaaa", vocab, 3)
	want := []int64{0, 43, 43, 43, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestSplitPhonemesCutsAtSpace(t *testing.T) {
	got := SplitPhonemes("aa bb cc", 5)
	want := []string{"aa", "bb cc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitPhonemes = %q, want %q", got, want)
	}
}

func TestSplitPhonemesHardCut(t *testing.T) {
	got := SplitPhonemes("abcdef", 4)
	want := []string{"abcd", "ef"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitPhonemes = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd tools/neural-tts && go test ./internal/phonemes -v`
Expected: FAIL — undefined types/functions.

- [ ] **Step 3: Implement**

`tools/neural-tts/internal/phonemes/phonemes.go`:

```go
// Package phonemes turns a line of text into model-ready token IDs.
package phonemes

import "strings"

// Clause is one punctuation-terminated segment of a line. Punct is the
// delimiter that followed it, or 0 for the final clause.
type Clause struct {
	Text  string
	Punct rune
}

var clauseDelimiters = map[rune]bool{
	',': true, '.': true, ';': true, ':': true,
	'!': true, '?': true, '—': true, '…': true,
}

// Split divides text at clause punctuation, keeping the delimiter so callers
// can re-attach it to the phoneme string (espeak drops punctuation).
func Split(text string) []Clause {
	var out []Clause
	var cur []rune

	flush := func(p rune) {
		t := strings.TrimSpace(string(cur))
		if t != "" {
			out = append(out, Clause{Text: t, Punct: p})
		}
		cur = cur[:0]
	}

	for _, r := range text {
		if clauseDelimiters[r] {
			flush(r)
			continue
		}
		cur = append(cur, r)
	}
	flush(0)
	return out
}

// IDs maps phoneme runes to vocabulary IDs, drops unknown runes, caps the
// payload at max, and wraps it in the 0 pad token the models expect.
func IDs(ps string, vocab map[string]int64, max int) []int64 {
	ids := make([]int64, 0, len(ps)+2)
	ids = append(ids, 0)
	n := 0
	for _, r := range ps {
		id, ok := vocab[string(r)]
		if !ok {
			continue
		}
		ids = append(ids, id)
		n++
		if n >= max {
			break
		}
	}
	return append(ids, 0)
}

// SplitPhonemes cuts a phoneme string into chunks of at most max runes,
// preferring the last space before the limit. This mirrors Paradee's _chunks.
func SplitPhonemes(ps string, max int) []string {
	rs := []rune(ps)
	var out []string
	for len(rs) > max {
		cut := max
		for i := max - 1; i > 0; i-- {
			if rs[i] == ' ' {
				cut = i
				break
			}
		}
		out = append(out, string(rs[:cut]))
		rs = []rune(strings.TrimLeft(string(rs[cut:]), " "))
	}
	if len(rs) > 0 {
		out = append(out, string(rs))
	}
	return out
}
```

- [ ] **Step 4: Run the tests**

Run: `cd tools/neural-tts && go test ./internal/phonemes -v`
Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/neural-tts/internal/phonemes
git commit -m "feat(neural-tts): clause splitting, token ids, and chunking"
```

---

## Task 4: Read Kokoro's voice style pack

**Files:**
- Create: `tools/neural-tts/internal/npz/npz_test.go`
- Create: `tools/neural-tts/internal/npz/npz.go`

- [ ] **Step 1: Write the failing test**

`tools/neural-tts/internal/npz/npz_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd tools/neural-tts && go test ./internal/npz -v`
Expected: FAIL — `undefined: Open`.

- [ ] **Step 3: Implement**

`tools/neural-tts/internal/npz/npz.go`:

```go
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
```

- [ ] **Step 4: Run the tests**

Run: `cd tools/neural-tts && go test ./internal/npz -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/neural-tts/internal/npz
git commit -m "feat(neural-tts): read Kokoro npy/npz voice styles"
```

---

## Task 5: WAV writer

**Files:**
- Create: `tools/neural-tts/internal/wav/wav_test.go`
- Create: `tools/neural-tts/internal/wav/wav.go`

- [ ] **Step 1: Write the failing test**

`tools/neural-tts/internal/wav/wav_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd tools/neural-tts && go test ./internal/wav -v`
Expected: FAIL — `undefined: Write`.

- [ ] **Step 3: Implement**

`tools/neural-tts/internal/wav/wav.go`:

```go
// Package wav writes mono 16-bit PCM WAV files.
package wav

import (
	"encoding/binary"
	"math"
	"os"
)

// Write clips samples to [-1,1] and writes a 16-bit PCM WAV.
func Write(path string, samples []float32, rate int) error {
	dataSize := len(samples) * 2
	buf := make([]byte, 44+dataSize)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:24], 1) // mono
	binary.LittleEndian.PutUint32(buf[24:28], uint32(rate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(rate*2))
	binary.LittleEndian.PutUint16(buf[32:34], 2)
	binary.LittleEndian.PutUint16(buf[34:36], 16)
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))

	for i, s := range samples {
		if math.IsNaN(float64(s)) {
			s = 0
		}
		if s > 1 {
			s = 1
		} else if s < -1 {
			s = -1
		}
		v := int16(math.Round(float64(s) * 32767))
		binary.LittleEndian.PutUint16(buf[44+i*2:46+i*2], uint16(v))
	}
	return os.WriteFile(path, buf, 0o644)
}
```

- [ ] **Step 4: Run the test**

Run: `cd tools/neural-tts && go test ./internal/wav -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/neural-tts/internal/wav
git commit -m "feat(neural-tts): write 16-bit PCM wav"
```

---

## Task 6: espeak-ng binding (runtime DLL)

**Files:**
- Create: `tools/neural-tts/internal/espeak/espeak_windows.go`
- Create: `tools/neural-tts/internal/espeak/espeak_other.go`
- Create: `tools/neural-tts/internal/espeak/espeak_test.go`

- [ ] **Step 1: Write the failing test**

`tools/neural-tts/internal/espeak/espeak_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd tools/neural-tts && go test ./internal/espeak -v`
Expected: FAIL — undefined `Init`, `Phonemize`.

- [ ] **Step 3: Implement the Windows binding**

`tools/neural-tts/internal/espeak/espeak_windows.go`:

```go
//go:build windows

// Package espeak wraps the bundled espeak-ng DLL with just enough API to
// produce IPA phonemes. The DLL is loaded at runtime, so building the helper
// does not require it to be present.
package espeak

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

const (
	audioOutputRetrieval = 1
	textModeUTF8         = 1
	phonemesIPA          = 0x02
)

var (
	mu      sync.Mutex
	loaded  bool
	initOK  bool
	initErr error

	procInit       *syscall.Proc
	procSetVoice   *syscall.Proc
	procToPhonemes *syscall.Proc
)

// Init loads and initializes espeak-ng. Subsequent calls are no-ops.
func Init(dllPath, dataDir string) error {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return initErr
	}
	loaded = true

	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		initErr = fmt.Errorf("load espeak dll: %w", err)
		return initErr
	}
	pInit, err := dll.FindProc("espeak_Initialize")
	if err != nil {
		initErr = err
		return initErr
	}
	pSet, err := dll.FindProc("espeak_SetVoiceByName")
	if err != nil {
		initErr = err
		return initErr
	}
	pPhon, err := dll.FindProc("espeak_TextToPhonemes")
	if err != nil {
		initErr = err
		return initErr
	}
	procInit, procSetVoice, procToPhonemes = pInit, pSet, pPhon

	dataPtr, err := syscall.BytePtrFromString(dataDir)
	if err != nil {
		initErr = err
		return initErr
	}
	rate, _, err := procInit.Call(
		uintptr(audioOutputRetrieval), 0, uintptr(unsafe.Pointer(dataPtr)), 0)
	if rate == 0 || rate == ^uintptr(0) {
		initErr = fmt.Errorf("espeak_Initialize failed: %v", err)
		return initErr
	}
	initOK = true
	return nil
}

// Phonemize returns the IPA phonemes for one clause. Punctuation is not
// included; callers re-attach it.
func Phonemize(text, lang string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if !initOK {
		return "", fmt.Errorf("espeak not initialized")
	}

	voicePtr, err := syscall.BytePtrFromString(lang)
	if err != nil {
		return "", err
	}
	if r, _, _ := procSetVoice.Call(uintptr(unsafe.Pointer(voicePtr))); r != 0 {
		return "", fmt.Errorf("espeak_SetVoiceByName(%q) failed", lang)
	}

	textPtr, err := syscall.BytePtrFromString(text)
	if err != nil {
		return "", err
	}
	ptr := unsafe.Pointer(textPtr)

	var out []byte
	for {
		raw, _, _ := procToPhonemes.Call(
			uintptr(unsafe.Pointer(&ptr)), textModeUTF8, phonemesIPA)
		if raw == 0 {
			break
		}
		s := goString(raw)
		if s == "" {
			break
		}
		if len(out) > 0 {
			out = append(out, ' ')
		}
		out = append(out, s...)
	}
	return string(out), nil
}

// goString reads a NUL-terminated C string without cgo.
func goString(p uintptr) string {
	var b []byte
	for i := uintptr(0); ; i++ {
		c := *(*byte)(unsafe.Pointer(p + i))
		if c == 0 {
			return string(b)
		}
		b = append(b, c)
	}
}
```

`tools/neural-tts/internal/espeak/espeak_other.go`:

```go
//go:build !windows

package espeak

import "fmt"

func Init(dllPath, dataDir string) error {
	return fmt.Errorf("espeak binding is not implemented on this platform yet")
}

func Phonemize(text, lang string) (string, error) {
	return "", fmt.Errorf("espeak binding is not implemented on this platform yet")
}
```

- [ ] **Step 4: Run the test**

Run: `cd tools/neural-tts && go test ./internal/espeak -v`
Expected: PASS (or SKIP if Piper has not been fetched).

- [ ] **Step 5: Commit**

```bash
git add tools/neural-tts/internal/espeak
git commit -m "feat(neural-tts): espeak-ng DLL binding"
```

---

## Task 7: ONNX engine for both families

**Files:**
- Create: `tools/neural-tts/internal/models/models.go`
- Create: `tools/neural-tts/internal/models/models_test.go`

- [ ] **Step 1: Add the ORT dependency**

```bash
cd tools/neural-tts
go get github.com/yalue/onnxruntime_go@v1.14.0
```

Expected: `go.mod` gains the require line; `go.sum` is written.

- [ ] **Step 2: Write the integration test**

`tools/neural-tts/internal/models/models_test.go`:

```go
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
```

- [ ] **Step 3: Implement the engine**

`tools/neural-tts/internal/models/models.go`:

```go
// Package models wraps the Paradee and Kokoro ONNX graphs behind one API.
package models

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/npz"
	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/phonemes"
)

// Family selects the voice-conditioning scheme.
type Family string

const (
	FamilyParadee Family = "paradee"
	FamilyKokoro  Family = "kokoro"
)

// Config is the subset of the model config the helper needs.
type Config struct {
	ModelType  string           `json:"model_type"`
	SampleRate int              `json:"sample_rate"`
	Voices     []string         `json:"voices"`
	Vocab      map[string]int64 `json:"vocab"`
}

// Engine owns one loaded model.
type Engine struct {
	family     Family
	config     *Config
	styles     *npz.Archive
	voiceIndex map[string]int64

	session     *ort.DynamicAdvancedSession
	inputNames  []string
	outputName  string
}

// New loads the config and creates the ONNX session. The ORT environment must
// already be initialized by the caller.
func New(family Family, modelPath, configPath, voicesPath string) (*Engine, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{SampleRate: 24000}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 24000
	}

	e := &Engine{
		family:     family,
		config:     cfg,
		voiceIndex: map[string]int64{},
	}
	for i, v := range cfg.Voices {
		e.voiceIndex[v] = int64(i)
	}

	if family == FamilyKokoro {
		e.styles, err = npz.Open(voicesPath)
		if err != nil {
			return nil, fmt.Errorf("open voices: %w", err)
		}
	}

	inputs, outputs, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("inspect model: %w", err)
	}
	if len(outputs) == 0 {
		return nil, fmt.Errorf("model has no outputs")
	}
	e.inputNames = make([]string, len(inputs))
	for i, in := range inputs {
		e.inputNames[i] = in.Name
	}
	e.outputName = outputs[0].Name

	e.session, err = ort.NewDynamicAdvancedSession(
		modelPath, e.inputNames, []string{e.outputName}, nil)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return e, nil
}

// SampleRate returns the model's output rate.
func (e *Engine) SampleRate() int { return e.config.SampleRate }

// Destroy releases the session.
func (e *Engine) Destroy() error {
	if e.session != nil {
		return e.session.Destroy()
	}
	return nil
}

// Voices lists the selectable voices for this family.
func (e *Engine) Voices() []string {
	if e.family == FamilyKokoro {
		var out []string
		for _, n := range e.styles.Names() {
			if isEnglishVoice(n) {
				out = append(out, n)
			}
		}
		sort.Strings(out)
		return out
	}
	out := make([]string, len(e.config.Voices))
	copy(out, e.config.Voices)
	return out
}

// IsEnglishVoice reports whether a Kokoro voice belongs to the English set.
func isEnglishVoice(name string) bool {
	if len(name) < 3 {
		return false
	}
	switch name[:2] {
	case "af", "am", "bf", "bm":
		return true
	}
	return false
}

// SynthesizePhonemes renders normalized phonemes to float32 samples. Long
// strings are cut at word boundaries into chunks of at most 510 phonemes.
func (e *Engine) SynthesizePhonemes(ps, voice string, speed float64) ([]float32, error) {
	var voiceIdx int64
	if e.family == FamilyParadee {
		idx, ok := e.voiceIndex[voice]
		if !ok {
			return nil, fmt.Errorf("unknown voice %q", voice)
		}
		voiceIdx = idx
	}
	if e.family == FamilyKokoro {
		if _, _, err := e.styles.Shape(voice); err != nil {
			return nil, err
		}
	}

	var audio []float32
	for _, chunk := range phonemes.SplitPhonemes(ps, 510) {
		ids := tokenIDs(chunk, e.config.Vocab)

		var style []float32
		if e.family == FamilyKokoro {
			rows, _, err := e.styles.Shape(voice)
			if err != nil {
				return nil, err
			}
			row := len(ids)
			if row > rows {
				row = rows
			}
			style, err = e.styles.Row(voice, row-1)
			if err != nil {
				return nil, err
			}
		}

		inputs, err := e.inputTensors(ids, style, voiceIdx, speed)
		if err != nil {
			return nil, err
		}
		outputs := []ort.Value{nil}
		if err := e.session.Run(inputs, outputs); err != nil {
			destroyAll(inputs)
			return nil, fmt.Errorf("run model: %w", err)
		}
		out, ok := outputs[0].(*ort.Tensor[float32])
		if !ok {
			_ = outputs[0].Destroy()
			destroyAll(inputs)
			return nil, fmt.Errorf("unexpected output type %T", outputs[0])
		}
		audio = append(audio, out.GetData()...)
		_ = out.Destroy()
		destroyAll(inputs)
	}
	return audio, nil
}

func tokenIDs(ps string, vocab map[string]int64) []int64 {
	ids := []int64{0}
	n := 0
	for _, r := range ps {
		id, ok := vocab[string(r)]
		if !ok {
			continue
		}
		ids = append(ids, id)
		n++
		if n >= 510 {
			break
		}
	}
	return append(ids, 0)
}

func (e *Engine) inputTensors(ids []int64, style []float32, voiceIdx int64, speed float64) ([]ort.Value, error) {
	inputs := make([]ort.Value, 0, len(e.inputNames))
	add := func(v ort.Value, err error) error {
		if err != nil {
			destroyAll(inputs)
			return err
		}
		inputs = append(inputs, v)
		return nil
	}
	for _, name := range e.inputNames {
		switch name {
		case "input_ids", "tokens":
			t, err := ort.NewTensor(ort.NewShape(1, int64(len(ids))), ids)
			if err := add(t, err); err != nil {
				return nil, err
			}
		case "speed":
			t, err := ort.NewTensor(ort.NewShape(1), []float32{float32(speed)})
			if err := add(t, err); err != nil {
				return nil, err
			}
		case "voice":
			t, err := ort.NewTensor(ort.NewShape(1), []int64{voiceIdx})
			if err := add(t, err); err != nil {
				return nil, err
			}
		case "style":
			if len(style) == 0 {
				return nil, fmt.Errorf("model needs style but no style row was loaded")
			}
			t, err := ort.NewTensor(ort.NewShape(1, int64(len(style))), style)
			if err := add(t, err); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported model input %q", name)
		}
	}
	return inputs, nil
}

func destroyAll(values []ort.Value) {
	for _, v := range values {
		if v != nil {
			_ = v.Destroy()
		}
	}
}
```

- [ ] **Step 4: Build**

Run: `cd tools/neural-tts && go build ./...`
Expected: builds with no output. (`go vet ./...` should also pass.)

- [ ] **Step 5: Run the integration tests (after Task 9 assets exist)**

Run:

```bash
cd tools/neural-tts
MDVIEWER_NEURAL_DIR=../../resources/neural go test ./internal/models -v
```

Expected: both tests PASS. If the assets are not downloaded yet, they SKIP; proceed to Task 9 and rerun.

- [ ] **Step 6: Commit**

```bash
git add tools/neural-tts/go.mod tools/neural-tts/go.sum tools/neural-tts/internal/models
git commit -m "feat(neural-tts): Onnx runtime engine for Paradee and Kokoro"
```

---

## Task 8: Wire the CLI

**Files:**
- Modify: `tools/neural-tts/main.go`

- [ ] **Step 1: Replace the stub with the full CLI**

`tools/neural-tts/main.go`:

```go
// Command neural-tts renders one line of text to a 24 kHz mono WAV using
// Paradee or Kokoro-82M. It is invoked per line by MDViewer and communicates
// through flags + stdin, mirroring the Piper helper.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/espeak"
	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/misaki"
	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/models"
	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/phonemes"
	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/wav"
)

type options struct {
	family     string
	modelPath  string
	configPath string
	voicesPath string
	voice      string
	speed      float64
	lang       string
	ortPath    string
	espeakDLL  string
	espeakData string
	outPath    string
	listVoices bool
}

func main() {
	var o options
	flag.StringVar(&o.family, "family", "paradee", "paradee|kokoro")
	flag.StringVar(&o.modelPath, "model", "", "path to the ONNX model")
	flag.StringVar(&o.configPath, "config", "", "path to config.json")
	flag.StringVar(&o.voicesPath, "voices", "", "Kokoro voices-v1.0.bin")
	flag.StringVar(&o.voice, "voice", "", "voice name")
	flag.Float64Var(&o.speed, "speed", 1.0, "1.0 = normal")
	flag.StringVar(&o.lang, "lang", "en-us", "en-us|en-gb")
	flag.StringVar(&o.ortPath, "ort", "", "path to onnxruntime.dll")
	flag.StringVar(&o.espeakDLL, "espeak-dll", "", "path to espeak-ng.dll")
	flag.StringVar(&o.espeakData, "espeak-data", "", "path to espeak-ng-data")
	flag.StringVar(&o.outPath, "out", "", "output WAV path")
	flag.BoolVar(&o.listVoices, "list-voices", false, "print voices and exit")
	flag.Parse()

	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "neural-tts:", err)
		os.Exit(1)
	}
}

func run(o options) error {
	family := models.Family(o.family)
	if family != models.FamilyParadee && family != models.FamilyKokoro {
		return fmt.Errorf("unknown family %q", o.family)
	}
	if o.configPath == "" {
		return fmt.Errorf("--config is required")
	}

	if o.listVoices {
		return listVoices(family, o.configPath, o.voicesPath)
	}
	if o.modelPath == "" || o.voice == "" || o.outPath == "" {
		return fmt.Errorf("--model, --voice, and --out are required")
	}

	if err := espeak.Init(o.espeakDLL, o.espeakData); err != nil {
		return err
	}
	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	ps, err := phonemize(string(text), o.lang)
	if err != nil {
		return err
	}
	if strings.TrimSpace(ps) == "" {
		return fmt.Errorf("no phonemes produced")
	}

	if o.ortPath == "" {
		o.ortPath = filepath.Join(filepath.Dir(os.Args[0]), "onnxruntime.dll")
	}
	ort.SetSharedLibraryPath(o.ortPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("init onnxruntime: %w", err)
	}
	defer ort.DestroyEnvironment()

	engine, err := models.New(family, o.modelPath, o.configPath, o.voicesPath)
	if err != nil {
		return err
	}
	defer engine.Destroy()

	audio, err := engine.SynthesizePhonemes(ps, o.voice, o.speed)
	if err != nil {
		return err
	}
	return wav.Write(o.outPath, audio, engine.SampleRate())
}

func phonemize(text, lang string) (string, error) {
	british := lang == "en-gb"
	var parts []string
	for _, c := range phonemes.Split(text) {
		raw, err := espeak.Phonemize(c.Text, lang)
		if err != nil {
			return "", err
		}
		ps := misaki.ToMisaki(raw, british)
		if c.Punct != 0 {
			ps += string(c.Punct)
		}
		parts = append(parts, ps)
	}
	return strings.Join(parts, " "), nil
}

func listVoices(family models.Family, configPath, voicesPath string) error {
	engine, err := models.ConfigOnly(family, configPath, voicesPath)
	if err != nil {
		return err
	}
	defer engine.Destroy()
	for _, v := range engine.Voices() {
		fmt.Printf("%s\t%s\t%s\n", family, v, label(string(family), v))
	}
	return nil
}

func label(family, key string) string {
	accent := map[string]string{
		"af": "en-US", "am": "en-US", "bf": "en-GB", "bm": "en-GB",
	}
	gender := ""
	if len(key) > 1 && key[1] == 'm' {
		gender = " (M)"
	} else if len(key) > 1 && key[1] == 'f' {
		gender = " (F)"
	}
	name := key
	if i := strings.IndexByte(key, '_'); i >= 0 && i+1 < len(key) {
		name = strings.ToUpper(key[i+1:i+2]) + key[i+2:]
	}
	loc := accent[key[:2]]
	if loc == "" {
		loc = "en"
	}
	switch family {
	case "paradee":
		return fmt.Sprintf("Paradee · %s — %s%s", name, loc, gender)
	case "kokoro":
		return fmt.Sprintf("Kokoro · %s — %s%s", name, loc, gender)
	}
	return fmt.Sprintf("%s · %s", family, name)
}
```

The CLI lists voices without loading a model, so add a config-only constructor
to `tools/neural-tts/internal/models/models.go`:

```go
// ConfigOnly loads just the config (and voice pack) for --list-voices.
func ConfigOnly(family Family, configPath, voicesPath string) (*Engine, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{SampleRate: 24000}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	e := &Engine{family: family, config: cfg, voiceIndex: map[string]int64{}}
	for i, v := range cfg.Voices {
		e.voiceIndex[v] = int64(i)
	}
	if family == FamilyKokoro {
		e.styles, err = npz.Open(voicesPath)
		if err != nil {
			return nil, fmt.Errorf("open voices: %w", err)
		}
	}
	return e, nil
}
```

`Engine.Destroy` already tolerates a nil session.

- [ ] **Step 2: Build**

Run: `cd tools/neural-tts && go build ./...`
Expected: builds with no output.

- [ ] **Step 3: Commit**

```bash
git add tools/neural-tts/main.go tools/neural-tts/internal/models/models.go
git commit -m "feat(neural-tts): CLI with list-voices and wav rendering"
```

---

## Task 9: Fetch assets and run the go/no-go spike

**Files:**
- Create: `resources/neural/paradee-config.json` (copy of the Paradee config)
- Create: `resources/neural/kokoro-config.json` (copy of the Kokoro config)
- Create: `resources/neural/` binaries (gitignored)

- [ ] **Step 1: Download the assets**

```bash
mkdir -p resources/neural
curl -L -o resources/neural/paradee_int8.onnx \
  "https://huggingface.co/sahilmahendrakar/Paradee-8M-v1.2/resolve/main/onnx/paradee_int8.onnx"
curl -L -o resources/neural/paradee-config.json \
  "https://huggingface.co/sahilmahendrakar/Paradee-8M-v1.2/resolve/main/config.json"
curl -L -o resources/neural/kokoro-v1.0.int8.onnx \
  "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/kokoro-v1.0.int8.onnx"
curl -L -o resources/neural/voices-v1.0.bin \
  "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/voices-v1.0.bin"
curl -L -o resources/neural/kokoro-config.json \
  "https://raw.githubusercontent.com/thewh1teagle/kokoro-onnx/main/src/kokoro_onnx/config.json"
powershell -ExecutionPolicy Bypass -File scripts/fetch-piper.ps1
```

- [ ] **Step 2: Verify hashes and extract ORT**

```bash
powershell -Command "Get-FileHash resources/neural/paradee_int8.onnx -Algorithm SHA256"
powershell -Command "Get-FileHash resources/neural/kokoro-v1.0.int8.onnx -Algorithm SHA256"
powershell -Command "Get-FileHash resources/neural/voices-v1.0.bin -Algorithm SHA256"
```

Expected: match the three hashes in Prerequisites.

```bash
curl -L -o /tmp/ort.zip "https://github.com/microsoft/onnxruntime/releases/download/v1.20.0/onnxruntime-win-x64-1.20.0.zip"
powershell -Command "Expand-Archive -Path '$env:TEMP\ort.zip' -DestinationPath '$env:TEMP\ort' -Force" 2>/dev/null || unzip -o /tmp/ort.zip -d /tmp/ort
cp /tmp/ort/onnxruntime-win-x64-1.20.0/lib/onnxruntime.dll resources/neural/
cp resources/piper/engine/windows/piper/espeak-ng.dll resources/neural/
```

- [ ] **Step 3: Render a sample with each family**

```bash
cd tools/neural-tts
go build -o ../../resources/neural/neural-tts.exe .

cd ../../resources/neural
echo "Paradee is a small voice that runs anywhere." | ./neural-tts.exe \
  --family paradee --model paradee_int8.onnx --config paradee-config.json \
  --voice af_heart --speed 1.0 --lang en-us \
  --ort onnxruntime.dll --espeak-dll espeak-ng.dll \
  --espeak-data ../piper/engine/windows/piper/espeak-ng-data \
  --out spike-paradee.wav

echo "Kokoro runs on the CPU and sounds natural." | ./neural-tts.exe \
  --family kokoro --model kokoro-v1.0.int8.onnx --config kokoro-config.json \
  --voices voices-v1.0.bin --voice af_heart --speed 1.0 --lang en-us \
  --ort onnxruntime.dll --espeak-dll espeak-ng.dll \
  --espeak-data ../piper/engine/windows/piper/espeak-ng-data \
  --out spike-kokoro.wav
```

Expected: both commands exit 0 and create non-empty WAV files.

- [ ] **Step 4: Verify the WAVs and measure**

```bash
powershell -Command "(New-Object System.Media.SoundPlayer 'spike-paradee.wav').PlaySync()"
powershell -Command "(New-Object System.Media.SoundPlayer 'spike-kokoro.wav').PlaySync()"
powershell -Command "Measure-Command { 1..3 | ForEach-Object { 'Hello world, this is a test.' | .\neural-tts.exe --family kokoro --model kokoro-v1.0.int8.onnx --config kokoro-config.json --voices voices-v1.0.bin --voice af_heart --speed 1.0 --lang en-us --ort onnxruntime.dll --espeak-dll espeak-ng.dll --espeak-data ../piper/engine/windows/piper/espeak-ng-data --out t.wav } } | Select-Object TotalSeconds"
```

Expected: intelligible speech for both; Kokoro warm render under ~1.5 s per line.

- [ ] **Step 5: Go/no-go checkpoint**

Ask the user to listen to `resources/neural/spike-paradee.wav` and
`resources/neural/spike-kokoro.wav`. If either is garbled, stop and debug the
phoneme pipeline before continuing. Do not proceed on audio that only
technically produces samples.

- [ ] **Step 6: Commit configs and test evidence**

```bash
git add resources/neural/paradee-config.json resources/neural/kokoro-config.json
git commit -m "chore(neural): add pinned Paradee and Kokoro configs"
```

---

## Task 10: App-side selector and speed helpers

**Files:**
- Modify: `internal/tts/tts.go`
- Modify: `internal/tts/tts_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tts/tts_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tts -run 'TestNeural' -v`
Expected: FAIL — undefined `NeuralVoiceID`, `NeuralSpeed`.

- [ ] **Step 3: Implement**

Append to `internal/tts/tts.go`:

```go
const (
	paradeePrefix = "paradee\t"
	kokoroPrefix  = "kokoro\t"
)

// NeuralVoiceID splits a Paradee or Kokoro selector.
func NeuralVoiceID(voice string) (family, id string, ok bool) {
	switch {
	case strings.HasPrefix(voice, paradeePrefix):
		return "paradee", strings.TrimPrefix(voice, paradeePrefix), true
	case strings.HasPrefix(voice, kokoroPrefix):
		return "kokoro", strings.TrimPrefix(voice, kokoroPrefix), true
	}
	return "", "", false
}

// NeuralSpeed maps the UI rate (100-250 WPM) to the models' speed input,
// where 1.0 is normal and values are clamped to [0.5, 2.0] (Kokoro's range).
func NeuralSpeed(rate int) float64 {
	if rate <= 0 {
		return 1.0
	}
	s := float64(rate) / 175.0
	if s < 0.5 {
		return 0.5
	}
	if s > 2.0 {
		return 2.0
	}
	return s
}

// NeuralBinaryName returns the helper filename for the given GOOS.
func NeuralBinaryName(goos string) string {
	if goos == "windows" {
		return "neural-tts.exe"
	}
	return "neural-tts"
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/tts -v`
Expected: all existing + new tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tts/tts.go internal/tts/tts_test.go
git commit -m "feat(tts): Paradee/Kokoro selectors and speed mapping"
```

---

## Task 11: App integration (`tts_neural.go`)

**Files:**
- Create: `tts_neural.go`
- Modify: `app.go` (App fields only)

- [ ] **Step 1: Add fields to App**

In `app.go`, extend the `App` struct:

```go
	neuralMu     sync.Mutex
	neuralAssets *neuralAssets
	neuralVoices []string
	neuralLoaded bool
```

- [ ] **Step 2: Create `tts_neural.go`**

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runtimePkg "runtime"
	"strings"

	"MDViewer/internal/tts"
)

// neuralAssets holds the resolved paths for the neural TTS helper. A family
// whose model files are missing is simply not listed.
type neuralAssets struct {
	dir        string
	exe        string
	ort        string
	espeakDLL  string
	espeakData string

	paradeeModel  string
	paradeeConfig string

	kokoroModel  string
	kokoroVoices string
	kokoroConfig string
}

// resolveNeural locates the helper and its assets, caching the result.
func (a *App) resolveNeural() *neuralAssets {
	a.neuralMu.Lock()
	defer a.neuralMu.Unlock()
	if a.neuralAssets != nil {
		return a.neuralAssets
	}

	var dirs []string
	if env := os.Getenv("MDVIEWER_NEURAL_DIR"); env != "" {
		dirs = append(dirs, env)
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		dirs = append(dirs, filepath.Join(exeDir, "neural"))
		if runtimePkg.GOOS == "darwin" {
			dirs = append(dirs, filepath.Join(exeDir, "..", "Resources", "neural"))
		}
	}

	binName := tts.NeuralBinaryName(runtimePkg.GOOS)
	ortName := "onnxruntime.dll"
	if runtimePkg.GOOS != "windows" {
		ortName = "libonnxruntime.dylib"
	}

	for _, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		exePath := filepath.Join(abs, binName)
		if _, err := os.Stat(exePath); err != nil {
			continue
		}
		ortPath := filepath.Join(abs, ortName)
		if _, err := os.Stat(ortPath); err != nil {
			continue
		}

		assets := &neuralAssets{
			dir:        abs,
			exe:        exePath,
			ort:        ortPath,
			espeakDLL:  firstExisting(filepath.Join(abs, "espeak-ng.dll"), filepath.Join(abs, "espeak-ng")),
			espeakData: firstExisting(
				filepath.Join(abs, "espeak-ng-data"),
				filepath.Join(abs, "..", "piper", "espeak-ng-data"),
			),
		}

		assets.paradeeModel = firstExisting(filepath.Join(abs, "paradee_int8.onnx"))
		assets.paradeeConfig = firstExisting(filepath.Join(abs, "paradee-config.json"))
		if assets.paradeeModel == "" || assets.paradeeConfig == "" {
			assets.paradeeModel, assets.paradeeConfig = "", ""
		}

		assets.kokoroModel = firstExisting(filepath.Join(abs, "kokoro-v1.0.int8.onnx"))
		assets.kokoroVoices = firstExisting(filepath.Join(abs, "voices-v1.0.bin"))
		assets.kokoroConfig = firstExisting(filepath.Join(abs, "kokoro-config.json"))
		if assets.kokoroModel == "" || assets.kokoroVoices == "" || assets.kokoroConfig == "" {
			assets.kokoroModel, assets.kokoroVoices, assets.kokoroConfig = "", "", ""
		}

		// Without espeak there is no G2P, so list no neural voices at all.
		if assets.espeakData == "" || assets.espeakDLL == "" {
			assets.paradeeModel, assets.paradeeConfig = "", ""
			assets.kokoroModel, assets.kokoroVoices, assets.kokoroConfig = "", "", ""
		}

		a.neuralAssets = assets
		return assets
	}
	a.neuralAssets = &neuralAssets{}
	return a.neuralAssets
}

func firstExisting(paths ...string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// getNeuralVoices lists Paradee and Kokoro voices via the helper. The result
// is cached for the app's lifetime.
func (a *App) getNeuralVoices() []string {
	a.neuralMu.Lock()
	if a.neuralLoaded {
		voices := a.neuralVoices
		a.neuralMu.Unlock()
		return voices
	}
	a.neuralLoaded = true
	a.neuralMu.Unlock()

	assets := a.resolveNeural()
	if assets.exe == "" {
		return nil
	}

	var out []string
	if assets.paradeeModel != "" {
		out = append(out, listNeuralVoices(assets, "paradee", assets.paradeeConfig, "")...)
	}
	if assets.kokoroModel != "" {
		out = append(out, listNeuralVoices(assets, "kokoro", assets.kokoroConfig, assets.kokoroVoices)...)
	}

	a.neuralMu.Lock()
	a.neuralVoices = out
	a.neuralMu.Unlock()
	return out
}

func listNeuralVoices(assets *neuralAssets, family, config, voices string) []string {
	args := []string{"--list-voices", "--family", family, "--config", config}
	if voices != "" {
		args = append(args, "--voices", voices)
	}
	cmd := exec.Command(assets.exe, args...)
	cmd.Dir = assets.dir
	hideProcessWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var result []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, family+"\t") {
			result = append(result, line)
		}
	}
	return result
}

// synthesizeNeuralFile renders one line with the neural helper.
func (a *App) synthesizeNeuralFile(family, voice, text string, rate int, entry *prefetchedAudio) (string, error) {
	assets := a.resolveNeural()
	if assets.exe == "" {
		return "", fmt.Errorf("neural engine not found")
	}

	var model, config, voices string
	switch family {
	case "paradee":
		model, config = assets.paradeeModel, assets.paradeeConfig
	case "kokoro":
		model, config, voices = assets.kokoroModel, assets.kokoroConfig, assets.kokoroVoices
	default:
		return "", fmt.Errorf("unknown neural family %q", family)
	}
	if model == "" || config == "" {
		return "", fmt.Errorf("neural %s assets not found", family)
	}

	lang := "en-us"
	if strings.HasPrefix(voice, "b") {
		lang = "en-gb"
	}

	tmp := ttsTempFile(".wav")
	args := []string{
		"--family", family,
		"--model", model,
		"--config", config,
		"--voice", voice,
		"--speed", fmt.Sprintf("%g", tts.NeuralSpeed(rate)),
		"--lang", lang,
		"--ort", assets.ort,
		"--espeak-data", assets.espeakData,
		"--out", tmp,
	}
	if voices != "" {
		args = append(args, "--voices", voices)
	}
	if runtimePkg.GOOS == "windows" {
		args = append(args, "--espeak-dll", assets.espeakDLL)
	}

	cmd := exec.Command(assets.exe, args...)
	cmd.Dir = assets.dir
	cmd.Stdin = strings.NewReader(text)
	hideProcessWindow(cmd)

	if err := a.runTracked(cmd, entry); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if _, err := os.Stat(tmp); err != nil {
		return "", fmt.Errorf("neural engine produced no audio: %w", err)
	}
	return tmp, nil
}
```

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: builds (the Wails app package also builds on Windows).

- [ ] **Step 4: Commit**

```bash
git add app.go tts_neural.go
git commit -m "feat(tts): resolve and invoke the neural helper"
```

---

## Task 12: Dispatch, voice listing, and Stop

**Files:**
- Modify: `tts_pipeline.go`
- Modify: `app.go`

- [ ] **Step 1: Add the neural branch to `synthesizeToFile`**

In `tts_pipeline.go`, replace the whole function with:

```go
func (a *App) synthesizeToFile(text, voice string, rate int, entry *prefetchedAudio) (string, error) {
	// selector is passed to the native engine. Natural-engine selectors are
	// cleared on failure so SAPI/say fall back to their default voice.
	selector := voice

	if id, ok := tts.PiperVoiceID(voice); ok {
		path, err := a.synthesizePiperFile(text, id, rate, entry)
		if err == nil {
			return path, nil
		}
		a.ttsMutex.Lock()
		stopped := a.stopRequested
		a.ttsMutex.Unlock()
		if stopped {
			return "", err
		}
		selector = ""
	} else if family, id, ok := tts.NeuralVoiceID(voice); ok {
		path, err := a.synthesizeNeuralFile(family, id, text, rate, entry)
		if err == nil {
			return path, nil
		}
		a.ttsMutex.Lock()
		stopped := a.stopRequested
		a.ttsMutex.Unlock()
		if stopped {
			return "", err
		}
		selector = ""
	}

	if runtimePkg.GOOS == "windows" {
		return a.synthesizeWindowsFile(text, selector, rate, entry)
	}
	return a.synthesizeMacOSFile(text, selector, rate, entry)
}
```

- [ ] **Step 2: Include neural voices in `GetAvailableVoices`**

In `app.go`:

```go
	return append(append(piper, a.getNeuralVoices()...), native...), nil
```

And in `refreshNativeVoicesAsync`:

```go
		voices := append(append(a.getPiperVoices(), a.getNeuralVoices()...), native...)
```

- [ ] **Step 3: Sweep the helper on macOS Stop**

In `Stop()`:

```go
		exec.Command("pkill", "-f", "piper").Run()
		exec.Command("pkill", "-f", "neural-tts").Run()
```

- [ ] **Step 4: Build and vet**

Run: `go build ./... && go vet ./...`
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add tts_pipeline.go app.go
git commit -m "feat(tts): dispatch neural voices and list them in the dropdown"
```

---

## Task 13: Frontend dropdown groups

**Files:**
- Modify: `frontend/src/main.js:1444-1504`

- [ ] **Step 1: Add the groups**

In `populateVoices`, after the `systemGroup` declaration, add:

```js
    const paradeeGroup = document.createElement('optgroup');
    paradeeGroup.label = 'Paradee';
    const kokoroGroup = document.createElement('optgroup');
    kokoroGroup.label = 'Kokoro';
```

Inside the `voices.forEach` chain, add a branch before the final `else`:

```js
        } else if (tabParts.length >= 3 && (tabParts[0] === 'paradee' || tabParts[0] === 'kokoro')) {
            option.value = tabParts[0] + '\t' + tabParts[1];
            option.textContent = tabParts[2];
            (tabParts[0] === 'paradee' ? paradeeGroup : kokoroGroup).appendChild(option);
```

After the existing `naturalGroup` append block, add:

```js
    if (paradeeGroup.children.length > 0) {
        paradeeGroup.label = 'Paradee (' + paradeeGroup.children.length + ')';
        voiceSelect.appendChild(paradeeGroup);
    }
    if (kokoroGroup.children.length > 0) {
        kokoroGroup.label = 'Kokoro (' + kokoroGroup.children.length + ')';
        voiceSelect.appendChild(kokoroGroup);
    }
```

Leave the default-selection block (`firstNative` / `firstBundled`) untouched.

- [ ] **Step 2: Build the frontend**

Run: `cd frontend && npm run build`
Expected: Vite build succeeds.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/main.js
git commit -m "feat(ui): add Paradee and Kokoro voice groups"
```

---

## Task 14: Fetch and build scripts

**Files:**
- Create: `scripts/fetch-neural.ps1`
- Modify: `build.bat`
- Modify: `.gitignore`

- [ ] **Step 1: Write the fetch script**

`scripts/fetch-neural.ps1`:

```powershell
# Downloads the pinned Paradee and Kokoro models plus ONNX Runtime 1.20.0.
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Dest = Join-Path $Root "resources\neural"
$Piper = Join-Path $Root "resources\piper\engine\windows\piper"
New-Item -ItemType Directory -Force -Path $Dest | Out-Null

function Get-Pinned($uri, $out, $sha) {
    if (Test-Path $out) {
        $existing = (Get-FileHash $out -Algorithm SHA256).Hash.ToLower()
        if ($existing -eq $sha) { Write-Host "present: $(Split-Path -Leaf $out)"; return }
        Remove-Item $out -Force
    }
    for ($i = 1; $i -le 3; $i++) {
        try { Invoke-WebRequest -Uri $uri -OutFile $out; break }
        catch { if ($i -eq 3) { throw }; Start-Sleep -Seconds 2 }
    }
    $got = (Get-FileHash $out -Algorithm SHA256).Hash.ToLower()
    if ($got -ne $sha) {
        Remove-Item $out -Force
        throw "checksum mismatch for $(Split-Path -Leaf $out): $got"
    }
}

Get-Pinned "https://huggingface.co/sahilmahendrakar/Paradee-8M-v1.2/resolve/main/onnx/paradee_int8.onnx" `
    (Join-Path $Dest "paradee_int8.onnx") `
    "a6989796692ac173597e15d405c971989202d5c73cfad65f22e90bd7b6c97da1"

Get-Pinned "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/kokoro-v1.0.int8.onnx" `
    (Join-Path $Dest "kokoro-v1.0.int8.onnx") `
    "6e742170d309016e5891a994e1ce1559c702a2ccd0075e67ef7157974f6406cb"

Get-Pinned "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/voices-v1.0.bin" `
    (Join-Path $Dest "voices-v1.0.bin") `
    "bca610b8308e8d99f32e6fe4197e7ec01679264efed0cac9140fe9c29f1fbf7d"

$OrtDll = Join-Path $Dest "onnxruntime.dll"
if (-not (Test-Path $OrtDll)) {
    $zip = Join-Path $env:TEMP "onnxruntime-1.20.0.zip"
    Get-Pinned "https://github.com/microsoft/onnxruntime/releases/download/v1.20.0/onnxruntime-win-x64-1.20.0.zip" `
        $zip "b372de85cedd9387a0d4386b982265e8420e5bcc2f29394317e76525b832942e"
    $tmp = Join-Path $env:TEMP "onnxruntime-1.20.0"
    if (Test-Path $tmp) { Remove-Item $tmp -Recurse -Force }
    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    Copy-Item (Join-Path $tmp "onnxruntime-win-x64-1.20.0\lib\onnxruntime.dll") $OrtDll
    Remove-Item $zip, $tmp -Recurse -Force
}

$espeak = Join-Path $Piper "espeak-ng.dll"
if (-not (Test-Path $espeak)) {
    throw "espeak-ng.dll missing; run scripts/fetch-piper.ps1 first"
}
Copy-Item $espeak (Join-Path $Dest "espeak-ng.dll") -Force

Write-Host "Neural assets ready in resources\neural\"
```

- [ ] **Step 2: Update `build.bat`**

After the Piper bundling block and before the final `echo OK`, insert:

```bat
echo Building neural TTS helper...
where gcc >nul 2>&1
if errorlevel 1 (
    echo [ERROR] gcc is required to build the neural TTS helper ^(MinGW^).
    exit /b 1
)
powershell -ExecutionPolicy Bypass -File "%~dp0scripts\fetch-neural.ps1"
if errorlevel 1 (
    echo [ERROR] Neural asset fetch failed.
    exit /b 1
)
set "NEURAL_OUT=%~dp0build\bin\neural"
if not exist "%NEURAL_OUT%" mkdir "%NEURAL_OUT%"
pushd "%~dp0tools\neural-tts"
set CGO_ENABLED=1
go build -o "%NEURAL_OUT%\neural-tts.exe" .
if errorlevel 1 (
    popd
    echo [ERROR] neural-tts build failed.
    exit /b 1
)
popd
copy /Y "%~dp0resources\neural\paradee_int8.onnx" "%NEURAL_OUT%\" >nul
copy /Y "%~dp0resources\neural\paradee-config.json" "%NEURAL_OUT%\" >nul
copy /Y "%~dp0resources\neural\kokoro-v1.0.int8.onnx" "%NEURAL_OUT%\" >nul
copy /Y "%~dp0resources\neural\voices-v1.0.bin" "%NEURAL_OUT%\" >nul
copy /Y "%~dp0resources\neural\kokoro-config.json" "%NEURAL_OUT%\" >nul
copy /Y "%~dp0resources\neural\onnxruntime.dll" "%NEURAL_OUT%\" >nul
copy /Y "%~dp0resources\neural\espeak-ng.dll" "%NEURAL_OUT%\" >nul
echo Neural TTS bundled at %NEURAL_OUT%
```

- [ ] **Step 3: Update `.gitignore`**

Append:

```gitignore
# Neural TTS models (fetched by scripts/fetch-neural.*)
resources/neural/*.onnx
resources/neural/*.bin
resources/neural/*.dll
resources/neural/*.exe
```

- [ ] **Step 4: Run the build**

Run: `build.bat`
Expected: `build/bin/MDViewer.exe` exists and `build/bin/neural/` contains
`neural-tts.exe`, both models, the voice pack, the configs, and both DLLs.

- [ ] **Step 5: Commit**

```bash
git add scripts/fetch-neural.ps1 build.bat .gitignore
git commit -m "build(neural): fetch, build, and bundle the neural helper"
```

---

## Task 15: Documentation and tracker

**Files:**
- Create: `Docs/NEURAL_TTS.md`
- Modify: `README.md`
- Modify: `Project-Tracker.md`

- [ ] **Step 1: Write `Docs/NEURAL_TTS.md`**

```markdown
# Neural TTS (Paradee and Kokoro)

MDViewer can read documents with two offline neural voices in addition to the
OS voices and Piper.

| Family | Model | Size | Voices |
|---|---|---|---|
| Paradee 1.2 | `paradee_int8.onnx` | 8.5 MB | 4 |
| Kokoro-82M | `kokoro-v1.0.int8.onnx` + `voices-v1.0.bin` | 121 MB | 28 English |

## How it works

```
text -> espeak-ng IPA -> misaki normalization -> token ids
     -> ONNX Runtime 1.20 -> 24 kHz WAV -> existing playback
```

The helper `tools/neural-tts` runs once per line, like Piper. MDViewer starts
it with the line on stdin and a temp WAV path, prefetches line N+1 while line N
plays, and kills the process tree on Stop.

## Build

1. `scripts/fetch-piper.ps1` (espeak-ng DLL + data)
2. `scripts/fetch-neural.ps1` (models, voice pack, ONNX Runtime)
3. `build.bat`

MinGW (`gcc`) is required for cgo. Without the neural assets the voices simply
do not appear.

## Manual test

Select "Paradee (4)" or "Kokoro (28)" in the voice dropdown, press Speak, and
verify prefetch, Stop, and fallback (rename `neural-tts.exe` and speak again).

## Licenses

- Paradee: Apache-2.0 (model and code, sahilmahendrakar/paradee).
- Kokoro-82M: Apache-2.0 (hexgrad/Kokoro-82M).
- kokoro-onnx export and voices: Apache-2.0 (thewh1teagle/kokoro-onnx).
- misaki mapping rules: Apache-2.0; ported from `web/misaki.js`.
- espeak-ng: GPL, bundled by the existing Piper engine.
- ONNX Runtime 1.20.0: MIT.
```

- [ ] **Step 2: Update the README voice section**

Add a short paragraph under the TTS section:

```markdown
### Neural voices (optional)

With `scripts/fetch-neural.ps1` run at build time, two offline neural engines
appear in the voice dropdown: **Paradee** (4 voices, 8.5 MB) and **Kokoro**
(28 English voices). Both run on the CPU through ONNX Runtime and reuse the
same prefetch/Stop pipeline as Piper. See `Docs/NEURAL_TTS.md`.
```

- [ ] **Step 3: Update `Project-Tracker.md`**

Add a `### 2026-11-01` bullet (same heading as the design entry):

```markdown
- **Neural TTS implemented (Paradee + Kokoro English).** Added the separate
  `tools/neural-tts` helper (espeak-ng → misaki → ONNX Runtime 1.20), the
  app-side `tts_neural.go` integration, dropdown groups, fetch/build scripts,
  and tests. Paradee (4 voices) and Kokoro (28 English voices) are optional;
  the native-voice default is unchanged. Verified: build.bat produces
  `build/bin/neural/`, both families render WAV, fallback and Stop behave.
```

- [ ] **Step 4: Commit**

```bash
git add Docs/NEURAL_TTS.md README.md Project-Tracker.md
git commit -m "docs: document the neural TTS engines"
```

---

## Task 16: Final verification

- [ ] **Step 1: Root module checks**

```bash
go vet ./...
go test ./...
```

Expected: PASS. (The root module does not import cgo.)

- [ ] **Step 2: Helper module checks**

```bash
cd tools/neural-tts
go vet ./...
MDVIEWER_NEURAL_DIR=../../resources/neural go test ./... -v
```

Expected: all tests PASS (integration tests run because the assets exist).

- [ ] **Step 3: Rebuild the app**

```bash
cd ../..
./build.bat
```

Expected: `build/bin/MDViewer.exe` and the bundled neural folder.

- [ ] **Step 4: Manual acceptance**

1. Launch `build\bin\MDViewer.exe`, open a markdown file.
2. Confirm the dropdown shows `Native voices (n)`, `Bundled voices (2)`,
   `Paradee (4)`, `Kokoro (28)`, and the default is still the first native voice.
3. Select `Paradee · Heart`, press Speak. Audio is intelligible, line
   highlighting progresses, and there is no gap between lines.
4. Press Stop mid-line. Audio stops immediately and no `neural-tts.exe`
   remains in Task Manager.
5. Select `Kokoro · Heart` and repeat.
6. Rename `build\bin\neural\neural-tts.exe`, restart, confirm the neural groups
   disappear and the app still speaks with native voices. Rename it back.

- [ ] **Step 5: Commit any fixes and record the result**

```bash
git add -A
git commit -m "test(neural): record manual acceptance of Paradee and Kokoro"
```

---

## Task 17: macOS (deferred, separate plan)

Do not start this task in the Windows effort. Phase 3 will:

- add `espeak_darwin.go` that invokes the Piper-bundled `espeak-ng` binary;
- build `neural-tts` for `darwin-arm64`/`darwin-amd64` in the macOS CI
  workflow and bundle it under `Contents/Resources/neural`;
- add `.paradee`/`.kokoro` kinds to `VoiceOption`, extend the rewritten
  `VoiceCatalog`, and add `renderNeural` to the AVSpeech-based `SpeechEngine`.

Write a separate plan after phases 1–2 are accepted.
