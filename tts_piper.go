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
