// Package tts holds pure, dependency-free helpers for MDViewer's text-to-speech
// engine selection. Keeping these out of the Wails app package makes them unit
// testable without GUI build dependencies.
package tts

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

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
