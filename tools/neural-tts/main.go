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
