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
	serve      bool
	in         io.Reader
	out        io.Writer
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
	flag.BoolVar(&o.serve, "serve", false, "keep the model loaded and serve JSON lines on stdin")
	flag.Parse()
	o.in = os.Stdin
	o.out = os.Stdout

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
	if o.serve {
		return runServe(o)
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
