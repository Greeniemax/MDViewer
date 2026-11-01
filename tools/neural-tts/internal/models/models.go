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

	session    *ort.DynamicAdvancedSession
	inputNames []string
	outputName string
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
