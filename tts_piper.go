package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runtimePkg "runtime"
	"sort"
	"strings"

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

// synthesizePiperFile renders one line to a complete WAV with Piper. The
// file-then-play flow guarantees the full audio tail (no cutoff).
func (a *App) synthesizePiperFile(text, voiceID string, rate int, entry *prefetchedAudio) (string, error) {
	binPath, voicesDir, ok := a.resolvePiper()
	if !ok {
		return "", fmt.Errorf("piper engine not found")
	}
	modelPath := filepath.Join(voicesDir, voiceID+".onnx")
	if _, err := os.Stat(modelPath); err != nil {
		return "", fmt.Errorf("piper voice not found: %s", voiceID)
	}

	tmpFile := ttsTempFile(".wav")

	cmd := exec.Command(binPath,
		"--model", modelPath,
		"--output_file", tmpFile,
		"--length_scale", fmt.Sprintf("%g", tts.LengthScale(rate)),
	)
	cmd.Dir = filepath.Dir(binPath) // so adjacent espeak-ng-data resolves
	cmd.Stdin = strings.NewReader(text)
	hideProcessWindow(cmd)

	if err := a.runTracked(cmd, entry); err != nil {
		os.Remove(tmpFile)
		return "", err
	}
	return tmpFile, nil
}
