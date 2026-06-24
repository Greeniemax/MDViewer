# Natural TTS via Bundled Piper — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace MDViewer's robotic OS TTS with natural-sounding, offline neural speech by bundling the open-source Piper engine as the default voice on Windows and macOS, keeping native voices as a fallback.

**Architecture:** Add a `piper\t<modelKey>` voice engine that `SpeakLine` dispatches to before the existing OS branch. Piper synthesizes a complete WAV per line (reusing the proven file-then-play flow that fixed the audio-cutoff bug), then it is played via `afplay` (macOS) or a hidden `SoundPlayer` (Windows). Pure logic (rate mapping, selector parsing, voice labeling, path candidates) lives in a new dependency-free `internal/tts` package so it is unit-testable without GUI build deps; all I/O and `exec` stays in `app.go`.

**Tech Stack:** Go 1.23, Wails v2.11.0, vanilla JS frontend; Piper neural TTS (rhasspy/piper) + HuggingFace `rhasspy/piper-voices`.

## Global Constraints

- Module path: `MDViewer` (so internal package import path is `MDViewer/internal/tts`).
- Go version floor: `go 1.23`.
- Platforms in scope: Windows (amd64) + macOS (Intel `amd64` and Apple Silicon `arm64`). The Go package MUST compile on all three (no Windows-only fields in shared files).
- TTS stays **line-by-line**: `SpeakLine(text string, voice string, rate int) error` signature is unchanged; `GetAvailableVoices() ([]string, error)` signature is unchanged. No new Wails-bound methods (avoids regenerating frontend bindings).
- Voice selector wire formats (unchanged + new):
  - Piper (new): backend emits `"piper\t<modelKey>\t<label>"`; frontend sends `"piper\t<modelKey>"`.
  - Windows native: `"sapi\t<id>\t<label>"` / `"onecore\t<id>\t<label>"` → frontend sends `"<engine>\t<id>"`.
  - macOS native: `"Name|Description"` → frontend sends bare `"Name"`.
- Rate slider range stays 100–250 WPM. Piper maps it via `length_scale = clamp(175/rate, 0.5, 2.0)`.
- Pinned Piper engine release: `2023.11.14-2`. Default bundled voices: `en_US-amy-medium`, `en_US-ryan-medium`.
- Frequent commits: one commit per task. Do not push (desktop app; no deploy step).

---

### Task 1: Pure dispatch logic — `internal/tts` (rate + selector)

**Files:**
- Create: `internal/tts/tts.go`
- Test: `internal/tts/tts_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func LengthScale(rate int) float64`
  - `func PiperVoiceID(voice string) (id string, ok bool)`

- [ ] **Step 1: Write the failing tests**

Create `internal/tts/tts_test.go`:

```go
package tts

import "testing"

func TestLengthScale(t *testing.T) {
	cases := []struct {
		rate int
		want float64
	}{
		{175, 1.0},     // natural pace
		{350, 0.5},     // clamped fast
		{50, 2.0},      // clamped slow (175/50=3.5 -> 2.0)
		{0, 1.0},       // guard against divide-by-zero
		{-10, 1.0},     // guard against negative
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tts/ -v`
Expected: FAIL — `undefined: LengthScale` / `undefined: PiperVoiceID` (build error).

- [ ] **Step 3: Write minimal implementation**

Create `internal/tts/tts.go`:

```go
// Package tts holds pure, dependency-free helpers for MDViewer's text-to-speech
// engine selection. Keeping these out of the Wails app package makes them unit
// testable without GUI build dependencies.
package tts

import "strings"

// piperPrefix marks a Piper voice selector: "piper\t<modelKey>".
const piperPrefix = "piper\t"

// LengthScale maps the UI speech rate (100-250 WPM) to Piper's --length_scale,
// where 1.0 is the natural pace, higher is slower, lower is faster. 175 WPM maps
// to 1.0. The result is clamped to [0.5, 2.0].
func LengthScale(rate int) float64 {
	if rate <= 0 {
		return 1.0
	}
	ls := 175.0 / float64(rate)
	if ls < 0.5 {
		return 0.5
	}
	if ls > 2.0 {
		return 2.0
	}
	return ls
}

// PiperVoiceID extracts the Piper model key from a voice selector and reports
// whether the selector is a Piper voice at all.
func PiperVoiceID(voice string) (string, bool) {
	if strings.HasPrefix(voice, piperPrefix) {
		return strings.TrimPrefix(voice, piperPrefix), true
	}
	return "", false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts/ -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tts/tts.go internal/tts/tts_test.go
git commit -m "feat(tts): add pure rate-mapping and piper selector parsing"
```

---

### Task 2: Pure resource logic — binary name, candidate dirs, voice label

**Files:**
- Modify: `internal/tts/tts.go`
- Test: `internal/tts/tts_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func BinaryName(goos string) string`
  - `func CandidateDirs(exeDir, goos string) []string`
  - `func VoiceLabel(configJSON []byte, modelKey string) string`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tts/tts_test.go`:

```go
import "path/filepath" // add to existing import block at top of file

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
```

(`strings` is already imported in the test file via Task 1? No — add `"strings"` to the test import block. Ensure the test file imports `testing`, `strings`, and `path/filepath`.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tts/ -v`
Expected: FAIL — `undefined: BinaryName` / `CandidateDirs` / `VoiceLabel`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/tts/tts.go` (and add `"encoding/json"`, `"path/filepath"` to its import block, keeping `"strings"`):

```go
// BinaryName returns the Piper executable filename for the given GOOS.
func BinaryName(goos string) string {
	if goos == "windows" {
		return "piper.exe"
	}
	return "piper"
}

// CandidateDirs returns ordered base directories that may hold the bundled Piper
// engine (each expected to contain the binary plus a voices/ subdir), given the
// running executable's directory and GOOS.
func CandidateDirs(exeDir, goos string) []string {
	dirs := []string{filepath.Join(exeDir, "piper")}
	if goos == "darwin" {
		// Inside an .app: Contents/MacOS/<exe> -> Contents/Resources/piper
		dirs = append(dirs, filepath.Join(exeDir, "..", "Resources", "piper"))
	}
	return dirs
}

// voiceConfig is the subset of a Piper .onnx.json we read for the UI label.
type voiceConfig struct {
	Dataset  string `json:"dataset"`
	Language struct {
		Code string `json:"code"`
	} `json:"language"`
}

// VoiceLabel builds a friendly dropdown label from a Piper voice's .onnx.json
// bytes, falling back to the model key when the JSON is missing or unparseable.
func VoiceLabel(configJSON []byte, modelKey string) string {
	name := modelKey
	code := ""
	var cfg voiceConfig
	if err := json.Unmarshal(configJSON, &cfg); err == nil {
		if cfg.Dataset != "" {
			name = cfg.Dataset
		}
		code = cfg.Language.Code
	}
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	if code != "" {
		return name + " (Natural) — " + code
	}
	return name + " (Natural)"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts/ -v`
Expected: PASS (all five tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tts/tts.go internal/tts/tts_test.go
git commit -m "feat(tts): add piper binary-name, path candidates, voice labeling"
```

---

### Task 3: Cross-platform hidden-window helper (fix macOS/Linux compile)

**Why:** `app.go` currently sets `syscall.SysProcAttr{HideWindow, CreationFlags}` inline. Those fields exist only on Windows, so the package does not compile on macOS/Linux. Move the OS-specific bit behind a build-tagged helper so the shared code compiles everywhere (required for macOS Piper builds).

**Files:**
- Create: `winproc_windows.go`
- Create: `winproc_other.go`
- Modify: `app.go` (remove `"syscall"` import; replace two `SysProcAttr` blocks)

**Interfaces:**
- Produces: `func hideProcessWindow(cmd *exec.Cmd)` (hides the console window on Windows; no-op elsewhere).
- Consumed by: `runHiddenPowerShell`, `getAvailableVoicesWindows` (this task) and `speakLinePiper`/`playWavFile` (Task 4).

- [ ] **Step 1: Create the Windows implementation**

Create `winproc_windows.go`:

```go
//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideProcessWindow prevents a child process from flashing a console window.
func hideProcessWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
```

- [ ] **Step 2: Create the non-Windows no-op**

Create `winproc_other.go`:

```go
//go:build !windows

package main

import "os/exec"

// hideProcessWindow is a no-op on platforms without Windows console windows.
func hideProcessWindow(cmd *exec.Cmd) {}
```

- [ ] **Step 3: Refactor `app.go` to use the helper**

In `app.go`, edit `runHiddenPowerShell` — replace:

```go
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
```

with:

```go
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	hideProcessWindow(cmd)
```

In `getAvailableVoicesWindows`, replace:

```go
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
```

with:

```go
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	hideProcessWindow(cmd)
```

Then remove the now-unused `"syscall"` line from the `app.go` import block.

- [ ] **Step 4: Verify it compiles for the host platform**

Run: `go vet .`
Expected: no errors (the package now compiles on this Linux/macOS host; previously it failed on `SysProcAttr.HideWindow`).
If the host lacks Wails GUI C-deps and `go vet .` errors on webkit/gtk rather than our code, instead run: `GOOS=windows go build ./... && GOOS=darwin go build ./...` — both must succeed.

- [ ] **Step 5: Commit**

```bash
git add winproc_windows.go winproc_other.go app.go
git commit -m "refactor: portable hidden-window helper so package builds on macOS"
```

---

### Task 4: Piper synthesis + playback + dispatch + stop

**Files:**
- Create: `tts_piper.go`
- Modify: `app.go` (`SpeakLine` dispatch; `Stop`)

**Interfaces:**
- Consumes: `tts.LengthScale`, `tts.PiperVoiceID`, `tts.BinaryName`, `tts.CandidateDirs` (Tasks 1–2); `hideProcessWindow` (Task 3); `App.ttsMutex`, `App.ttsProcess` (existing).
- Produces (methods on `*App`):
  - `func (a *App) resolvePiper() (binPath, voicesDir string, ok bool)`
  - `func (a *App) speakLinePiper(text, voiceID string, rate int) error`
  - `func (a *App) playWavFile(path string) error`

- [ ] **Step 1: Create `tts_piper.go`**

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runtimePkg "runtime"
	"strings"
	"time"

	"MDViewer/internal/tts"
)

// resolvePiper locates the bundled Piper binary and its voices directory.
// Honors the MDVIEWER_PIPER_DIR override (which must contain the binary and a
// voices/ subdir), then falls back to locations next to the executable.
func (a *App) resolvePiper() (string, string, bool) {
	binName := tts.BinaryName(runtimePkg.GOOS)

	if env := os.Getenv("MDVIEWER_PIPER_DIR"); env != "" {
		bp := filepath.Join(env, binName)
		if _, err := os.Stat(bp); err == nil {
			return bp, filepath.Join(env, "voices"), true
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return "", "", false
	}
	exeDir := filepath.Dir(exe)
	for _, base := range tts.CandidateDirs(exeDir, runtimePkg.GOOS) {
		bp := filepath.Join(base, binName)
		if _, err := os.Stat(bp); err == nil {
			return bp, filepath.Join(base, "voices"), true
		}
	}
	return "", "", false
}

// speakLinePiper synthesizes one line to a complete WAV with Piper, then plays
// it. The file-then-play flow guarantees the full audio tail (no cutoff).
func (a *App) speakLinePiper(text, voiceID string, rate int) error {
	binPath, voicesDir, ok := a.resolvePiper()
	if !ok {
		return fmt.Errorf("piper engine not found")
	}
	modelPath := filepath.Join(voicesDir, voiceID+".onnx")
	if _, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("piper voice not found: %s", voiceID)
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("mdviewer_tts_%d.wav", time.Now().UnixNano()))

	cmd := exec.Command(binPath,
		"--model", modelPath,
		"--output_file", tmpFile,
		"--length_scale", fmt.Sprintf("%g", tts.LengthScale(rate)),
	)
	cmd.Dir = filepath.Dir(binPath) // so adjacent espeak-ng-data resolves
	cmd.Stdin = strings.NewReader(text)
	hideProcessWindow(cmd)

	a.ttsMutex.Lock()
	a.ttsProcess = cmd
	a.ttsMutex.Unlock()

	if err := cmd.Run(); err != nil {
		os.Remove(tmpFile)
		return err
	}

	err := a.playWavFile(tmpFile)
	os.Remove(tmpFile)
	return err
}

// playWavFile plays a WAV to completion: afplay on macOS, a hidden SoundPlayer
// on Windows (no console flash, consistent with existing Windows playback).
func (a *App) playWavFile(path string) error {
	var cmd *exec.Cmd
	if runtimePkg.GOOS == "windows" {
		escaped := strings.ReplaceAll(path, "'", "''")
		ps := fmt.Sprintf("(New-Object System.Media.SoundPlayer '%s').PlaySync()", escaped)
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
		hideProcessWindow(cmd)
	} else {
		cmd = exec.Command("afplay", path)
	}

	a.ttsMutex.Lock()
	a.ttsProcess = cmd
	a.ttsMutex.Unlock()

	return cmd.Run()
}
```

- [ ] **Step 2: Wire dispatch into `SpeakLine`**

In `app.go`, replace this block inside `SpeakLine`:

```go
	var err error
	if runtimePkg.GOOS == "windows" {
		err = a.speakLineWindows(text, voice, rate)
	} else {
		err = a.speakLineMacOS(text, voice, rate)
	}
```

with:

```go
	var err error
	if id, ok := tts.PiperVoiceID(voice); ok {
		err = a.speakLinePiper(text, id, rate)
		if err != nil {
			// Natural engine failed — fall back to the platform-native voice.
			if runtimePkg.GOOS == "windows" {
				err = a.speakLineWindows(text, "", rate)
			} else {
				err = a.speakLineMacOS(text, "", rate)
			}
		}
	} else if runtimePkg.GOOS == "windows" {
		err = a.speakLineWindows(text, voice, rate)
	} else {
		err = a.speakLineMacOS(text, voice, rate)
	}
```

Add `"MDViewer/internal/tts"` to the `app.go` import block.

- [ ] **Step 3: Extend `Stop` to kill Piper**

In `app.go` `Stop`, in the Windows branch add (alongside the existing `taskkill`):

```go
			exec.Command("taskkill", "/F", "/IM", "piper.exe").Run()
```

and in the non-Windows branch add (alongside the existing `pkill` calls):

```go
			exec.Command("pkill", "-f", "piper").Run()
```

- [ ] **Step 4: Verify it compiles**

Run: `go vet .`
Expected: no errors.
(If the host lacks Wails GUI C-deps, instead run `GOOS=windows go build ./... && GOOS=darwin go build ./...`; both must succeed.)
Also re-run `go test ./internal/tts/ -v` → still PASS.

- [ ] **Step 5: Commit**

```bash
git add tts_piper.go app.go
git commit -m "feat(tts): add piper synthesis, playback, dispatch, and stop"
```

---

### Task 5: Offer Piper voices from `GetAvailableVoices`

**Files:**
- Modify: `app.go` (`GetAvailableVoices`; add `getPiperVoices`)

**Interfaces:**
- Consumes: `resolvePiper` (Task 4), `tts.VoiceLabel` (Task 2).
- Produces: `GetAvailableVoices` now returns Piper voices first (`"piper\t<key>\t<label>"`), then existing native voices.

- [ ] **Step 1: Add `getPiperVoices` to `tts_piper.go`**

Add `"sort"` to the `tts_piper.go` import block, then append:

```go
// getPiperVoices lists bundled Piper voices as "piper\t<modelKey>\t<label>".
func (a *App) getPiperVoices() []string {
	_, voicesDir, ok := a.resolvePiper()
	if !ok {
		return nil
	}
	entries, err := os.ReadDir(voicesDir)
	if err != nil {
		return nil
	}
	var voices []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".onnx") {
			continue
		}
		modelKey := strings.TrimSuffix(name, ".onnx")
		label := modelKey + " (Natural)"
		if data, err := os.ReadFile(filepath.Join(voicesDir, name+".json")); err == nil {
			label = tts.VoiceLabel(data, modelKey)
		}
		voices = append(voices, "piper\t"+modelKey+"\t"+label)
	}
	sort.Strings(voices)
	return voices
}
```

- [ ] **Step 2: Prepend Piper voices in `GetAvailableVoices`**

In `app.go`, replace the body of `GetAvailableVoices`:

```go
func (a *App) GetAvailableVoices() ([]string, error) {
	if runtimePkg.GOOS == "windows" {
		return a.getAvailableVoicesWindows()
	}
	return a.getAvailableVoicesMacOS()
}
```

with:

```go
func (a *App) GetAvailableVoices() ([]string, error) {
	piper := a.getPiperVoices()

	var native []string
	var err error
	if runtimePkg.GOOS == "windows" {
		native, err = a.getAvailableVoicesWindows()
	} else {
		native, err = a.getAvailableVoicesMacOS()
	}
	if err != nil && len(piper) == 0 {
		return nil, err
	}
	return append(piper, native...), nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go vet .` (or `GOOS=windows go build ./... && GOOS=darwin go build ./...`).
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add tts_piper.go app.go
git commit -m "feat(tts): surface bundled piper voices in the voice list"
```

---

### Task 6: Group the voice dropdown (Natural on top, default to Piper)

**Files:**
- Modify: `frontend/src/main.js` (`loadVoices`, ~lines 1284–1312)

**Interfaces:**
- Consumes: `GetAvailableVoices` returning `"piper\t<key>\t<label>"` first, then native entries.
- Produces: a dropdown with `<optgroup label="Natural">` and `<optgroup label="System voices">`, defaulting to the first Piper voice. Selected `value` formats are unchanged for native voices and `"piper\t<key>"` for Piper.

- [ ] **Step 1: Replace the `loadVoices` function**

Replace the existing `loadVoices` function with:

```js
async function loadVoices() {
    try {
        const voices = await GetAvailableVoices();
        voiceSelect.innerHTML = '';

        const defaultOpt = document.createElement('option');
        defaultOpt.value = '';
        defaultOpt.textContent = 'Default Voice';
        voiceSelect.appendChild(defaultOpt);

        const naturalGroup = document.createElement('optgroup');
        naturalGroup.label = 'Natural';
        const systemGroup = document.createElement('optgroup');
        systemGroup.label = 'System voices';

        voices.forEach(voice => {
            const option = document.createElement('option');
            const tabParts = voice.split('\t');
            if (tabParts.length >= 3 && tabParts[0] === 'piper') {
                option.value = 'piper\t' + tabParts[1];
                option.textContent = tabParts[2];
                naturalGroup.appendChild(option);
            } else if (tabParts.length >= 3 && (tabParts[0] === 'sapi' || tabParts[0] === 'onecore')) {
                option.value = tabParts[0] + '\t' + tabParts[1];
                option.textContent = tabParts[2];
                systemGroup.appendChild(option);
            } else {
                const sepIndex = voice.indexOf('|');
                if (sepIndex > 0) {
                    option.value = voice.substring(0, sepIndex);
                    option.textContent = voice.substring(0, sepIndex) + ' — ' + voice.substring(sepIndex + 1);
                } else {
                    option.value = voice;
                    option.textContent = voice;
                }
                systemGroup.appendChild(option);
            }
        });

        if (naturalGroup.children.length > 0) voiceSelect.appendChild(naturalGroup);
        if (systemGroup.children.length > 0) voiceSelect.appendChild(systemGroup);

        // Prefer a natural (Piper) voice by default.
        if (naturalGroup.children.length > 0) {
            voiceSelect.value = naturalGroup.children[0].value;
        }
    } catch (err) {
        console.error('Error loading voices:', err);
    }
}
```

- [ ] **Step 2: Build the frontend**

Run: `cd frontend && npm run build`
Expected: build succeeds, no JS errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/main.js
git commit -m "feat(ui): group voices and default to natural piper voice"
```

---

### Task 7: Fetch scripts for the Piper engine + voices

**Files:**
- Create: `scripts/fetch-piper.sh`
- Create: `scripts/fetch-piper.ps1`

**Interfaces:**
- Produces a populated `resources/piper/` tree:
  - `resources/piper/engine/{windows,darwin-amd64,darwin-arm64}/piper/` (binary + libs + `espeak-ng-data/`)
  - `resources/piper/voices/<modelKey>.onnx` + `<modelKey>.onnx.json`
- Consumed by: the build scripts (Task 8).

- [ ] **Step 1: Create `scripts/fetch-piper.sh`**

```bash
#!/usr/bin/env bash
# Downloads the pinned Piper engine (per OS) and default voices into resources/.
set -euo pipefail

PIPER_VERSION="2023.11.14-2"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENG="$ROOT/resources/piper/engine"
VOICES="$ROOT/resources/piper/voices"
REL="https://github.com/rhasspy/piper/releases/download/$PIPER_VERSION"
HF="https://huggingface.co/rhasspy/piper-voices/resolve/main"
mkdir -p "$ENG" "$VOICES"

dl() { curl -fL --retry 3 -o "$1" "$2"; }

fetch_engine_tar() { # target asset
  local target="$1" asset="$2"
  if [ -e "$ENG/$target/piper/piper" ]; then echo "engine $target present"; return; fi
  local tmp; tmp="$(mktemp -d)"
  dl "$tmp/p.tgz" "$REL/$asset"
  mkdir -p "$ENG/$target"
  tar -xzf "$tmp/p.tgz" -C "$ENG/$target"   # extracts a top-level piper/ dir
  rm -rf "$tmp"
}

fetch_engine_zip() { # target asset
  local target="$1" asset="$2"
  if [ -e "$ENG/$target/piper/piper.exe" ]; then echo "engine $target present"; return; fi
  if ! command -v unzip >/dev/null 2>&1; then
    echo "skip $target (unzip not available)"; return; fi
  local tmp; tmp="$(mktemp -d)"
  dl "$tmp/p.zip" "$REL/$asset"
  mkdir -p "$ENG/$target"
  unzip -o "$tmp/p.zip" -d "$ENG/$target" >/dev/null   # extracts a top-level piper/ dir
  rm -rf "$tmp"
}

fetch_voice() { # modelKey hf-subpath
  local key="$1" sub="$2"
  if [ -e "$VOICES/$key.onnx" ]; then echo "voice $key present"; return; fi
  dl "$VOICES/$key.onnx"      "$HF/$sub/$key.onnx?download=true"
  dl "$VOICES/$key.onnx.json" "$HF/$sub/$key.onnx.json?download=true"
}

fetch_engine_tar "darwin-amd64" "piper_macos_x64.tar.gz"
fetch_engine_tar "darwin-arm64" "piper_macos_aarch64.tar.gz"
fetch_engine_zip "windows"      "piper_windows_amd64.zip"

fetch_voice "en_US-amy-medium"  "en/en_US/amy/medium"
fetch_voice "en_US-ryan-medium" "en/en_US/ryan/medium"

echo "Piper assets ready in resources/piper/"
```

Then: `chmod +x scripts/fetch-piper.sh`

- [ ] **Step 2: Create `scripts/fetch-piper.ps1`**

```powershell
# Downloads the pinned Piper engine (Windows) and default voices into resources/.
$ErrorActionPreference = "Stop"
$PiperVersion = "2023.11.14-2"
$Root   = Split-Path -Parent $PSScriptRoot
$Eng    = Join-Path $Root "resources\piper\engine"
$Voices = Join-Path $Root "resources\piper\voices"
$Rel    = "https://github.com/rhasspy/piper/releases/download/$PiperVersion"
$Hf     = "https://huggingface.co/rhasspy/piper-voices/resolve/main"
New-Item -ItemType Directory -Force -Path $Eng, $Voices | Out-Null

function Fetch-Voice($key, $sub) {
    $onnx = Join-Path $Voices "$key.onnx"
    if (Test-Path $onnx) { Write-Host "voice $key present"; return }
    Invoke-WebRequest -Uri "$Hf/$sub/$key.onnx?download=true"      -OutFile $onnx
    Invoke-WebRequest -Uri "$Hf/$sub/$key.onnx.json?download=true" -OutFile (Join-Path $Voices "$key.onnx.json")
}

$winDir = Join-Path $Eng "windows"
if (-not (Test-Path (Join-Path $winDir "piper\piper.exe"))) {
    New-Item -ItemType Directory -Force -Path $winDir | Out-Null
    $zip = Join-Path $env:TEMP "piper_win.zip"
    Invoke-WebRequest -Uri "$Rel/piper_windows_amd64.zip" -OutFile $zip
    Expand-Archive -Path $zip -DestinationPath $winDir -Force   # creates piper\ subdir
    Remove-Item $zip
} else { Write-Host "engine windows present" }

Fetch-Voice "en_US-amy-medium"  "en/en_US/amy/medium"
Fetch-Voice "en_US-ryan-medium" "en/en_US/ryan/medium"

Write-Host "Piper assets ready in resources\piper\"
```

- [ ] **Step 3: Run the fetch (host platform) to verify it works**

Run (macOS/Linux): `bash scripts/fetch-piper.sh`
Expected: `resources/piper/voices/en_US-amy-medium.onnx` exists and the host engine dir is populated (`resources/piper/engine/darwin-arm64/piper/piper` on Apple Silicon).
(On Windows run `powershell -ExecutionPolicy Bypass -File scripts\fetch-piper.ps1`.)

- [ ] **Step 4: Commit**

```bash
git add scripts/fetch-piper.sh scripts/fetch-piper.ps1
git commit -m "build: add scripts to fetch pinned piper engine and voices"
```

---

### Task 8: Bundle Piper into builds + gitignore + resources README

**Files:**
- Modify: `build.sh`
- Modify: `build.bat`
- Modify: `.gitignore`
- Create: `resources/piper/README.md`

**Interfaces:**
- Consumes: the `resources/piper/` tree from Task 7; runtime path expectations from `resolvePiper` (Task 4):
  - Windows: `<exeDir>/piper/piper.exe` + `<exeDir>/piper/voices/`
  - macOS: `<app>/Contents/Resources/piper/piper` + `.../piper/voices/`
- Produces: builds that ship Piper next to the executable / inside the `.app`.

- [ ] **Step 1: Ignore the downloaded binaries**

Append to `.gitignore`:

```
# Piper TTS engine + voice models (fetched by scripts/fetch-piper.*)
resources/piper/engine/
resources/piper/voices/
```

- [ ] **Step 2: Document the resources layout**

Create `resources/piper/README.md`:

```markdown
# Bundled Piper TTS assets

Contents here are downloaded by `scripts/fetch-piper.sh` / `.ps1` and are
**not** committed (see `.gitignore`).

Layout:
- `engine/<target>/piper/` — Piper binary + libs + `espeak-ng-data/`
  (`<target>` ∈ `windows`, `darwin-amd64`, `darwin-arm64`)
- `voices/<modelKey>.onnx` (+ `.onnx.json`) — neural voice models (shared across platforms)

The build scripts copy the correct-OS engine and the voices into the build
output so MDViewer can speak with natural voices offline.
```

- [ ] **Step 3: Bundle into the macOS build (`build.sh`)**

In `build.sh`, after the `wails build` success check (right after the `✅ Build successful!` echo, before the final messages), insert:

```bash
# --- Bundle Piper into the .app ---
echo "🔊 Bundling Piper TTS engine + voices..."
bash "$(dirname "$0")/scripts/fetch-piper.sh"

case "$(uname -m)" in
  arm64|aarch64) PIPER_TARGET="darwin-arm64" ;;
  *)             PIPER_TARGET="darwin-amd64" ;;
esac

APP_RES="build/bin/MDViewer.app/Contents/Resources/piper"
mkdir -p "$APP_RES/voices"
cp -R "resources/piper/engine/$PIPER_TARGET/piper/." "$APP_RES/"
cp resources/piper/voices/*.onnx resources/piper/voices/*.onnx.json "$APP_RES/voices/"
# Local dev: clear quarantine so Gatekeeper allows the bundled binary.
xattr -dr com.apple.quarantine "$APP_RES" 2>/dev/null || true
echo "   Piper bundled at $APP_RES"
```

- [ ] **Step 4: Bundle into the Windows build (`build.bat`)**

In `build.bat`, after the successful `wails build -s` (before the final `echo OK:` line), insert:

```bat
echo Bundling Piper TTS engine + voices...
powershell -ExecutionPolicy Bypass -File "%~dp0scripts\fetch-piper.ps1"
if errorlevel 1 (
    echo [ERROR] Piper fetch failed.
    exit /b 1
)
set "PIPER_OUT=%~dp0build\bin\piper"
if not exist "%PIPER_OUT%\voices" mkdir "%PIPER_OUT%\voices"
xcopy /E /I /Y "%~dp0resources\piper\engine\windows\piper" "%PIPER_OUT%" >nul
copy /Y "%~dp0resources\piper\voices\*.onnx" "%PIPER_OUT%\voices\" >nul
copy /Y "%~dp0resources\piper\voices\*.onnx.json" "%PIPER_OUT%\voices\" >nul
echo Piper bundled at %PIPER_OUT%
```

- [ ] **Step 5: Verify the bundle end-to-end on the host**

Run (macOS): `./build.sh` then `./run.sh`, open `test_sample.md`, pick a "Natural" voice, and press Speak.
Expected: natural-sounding speech; problem words ("MicroPython", "changes", "programming") play with complete endings; Stop halts mid-line; selecting a "System voices" entry still works.
(Windows: `build.bat` then run `build\bin\MDViewer.exe`.)
Quick non-GUI sanity check of the engine itself:
`echo "MicroPython changes" | resources/piper/engine/<target>/piper/piper --model resources/piper/voices/en_US-amy-medium.onnx --output_file /tmp/t.wav && afplay /tmp/t.wav`

- [ ] **Step 6: Commit**

```bash
git add .gitignore resources/piper/README.md build.sh build.bat
git commit -m "build: bundle piper engine + voices into windows and macOS builds"
```

---

### Task 9: Documentation

**Files:**
- Create: `Docs/PIPER_TTS.md`
- Modify: `README.md` (TTS / requirements / build sections)
- Modify: `Project-Tracker.md` (today's log)

**Interfaces:** none (docs only).

- [ ] **Step 1: Write `Docs/PIPER_TTS.md`**

Document: what Piper is and why (natural, offline, free); the `piper\t` selector format; rate→`length_scale` mapping; the bundle layout and runtime path resolution (`resolvePiper`, `MDVIEWER_PIPER_DIR` override); how to add a voice (drop `.onnx`+`.onnx.json` in `resources/piper/voices/` and extend `fetch-piper.*`); and the macOS signing/quarantine requirement for distribution.

- [ ] **Step 2: Update `README.md`**

In the TTS feature description, state that MDViewer now uses natural neural voices via a bundled Piper engine (offline, no API key), with system voices available as a fallback. In requirements/build, note that building from source runs `scripts/fetch-piper.*` (needs internet once) and that macOS distribution builds must code-sign/notarize the bundled `piper` binary.

- [ ] **Step 3: Update `Project-Tracker.md`**

Under `### 2026-06-24`, add a bullet noting the Piper natural-TTS feature was implemented (engine bundled for Windows + macOS, native voices kept as fallback) and reference this plan.

- [ ] **Step 4: Commit**

```bash
git add Docs/PIPER_TTS.md README.md Project-Tracker.md
git commit -m "docs: document bundled piper natural TTS"
```

---

## Self-Review

**Spec coverage:**
- Engine = Piper, default → Tasks 1,4,6. Bundle-in-app (Win+mac Intel/ARM) → Tasks 7,8. Native voices kept as fallback → Tasks 5,6 (+ runtime fallback in Task 4). File-then-play (no cutoff) → Task 4. Rate mapping → Task 1. `piper\t` dispatch → Task 4. Voice list/optgroup → Tasks 5,6. Error fallback + Stop + macOS Gatekeeper → Tasks 4,8,9. Testing (pure unit tests + manual) → Tasks 1–2 (unit), 4/6/8 (manual). Two bundled voices → Tasks 7,8. The macOS-compile fix (Task 3) is an added prerequisite the spec implied via the "must compile on Windows + macOS" platform requirement. ✅ All covered.

**Placeholder scan:** No TBD/TODO/"handle edge cases"/"similar to" — every code step has complete code. ✅

**Type consistency:** `LengthScale`, `PiperVoiceID`, `BinaryName`, `CandidateDirs`, `VoiceLabel` (package `tts`) are defined in Tasks 1–2 and consumed with matching signatures in Task 4–5. `hideProcessWindow(*exec.Cmd)` defined in Task 3, used in Tasks 3–4. `resolvePiper`/`speakLinePiper`/`playWavFile`/`getPiperVoices` defined and used consistently. Selector format `"piper\t<key>"` consistent between backend emit (Task 5), dispatch (Task 4), and frontend (Task 6). ✅
