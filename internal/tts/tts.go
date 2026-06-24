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
