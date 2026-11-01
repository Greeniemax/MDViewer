// Package main's serve mode keeps one model in memory and renders lines on
// demand, which avoids reloading a large ONNX session per line.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/espeak"
	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/models"
	"github.com/Greeniemax/MDViewer/tools/neural-tts/internal/wav"
)

// serveRequest is one newline-delimited JSON request in --serve mode.
type serveRequest struct {
	Text  string  `json:"text"`
	Voice string  `json:"voice"`
	Speed float64 `json:"speed"`
	Lang  string  `json:"lang"`
	Out   string  `json:"out"`
}

// serveResponse is the reply to one serveRequest.
type serveResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// serveLoop reads requests from r and writes one response per request to w.
// A bad request or synthesis error is reported without stopping the server.
func serveLoop(r io.Reader, w io.Writer, synth func(serveRequest) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)

	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req serveRequest
		var resp serveResponse
		if err := json.Unmarshal(line, &req); err != nil {
			resp.Error = "bad request: " + err.Error()
		} else if err := synth(req); err != nil {
			resp.Error = err.Error()
		} else {
			resp.OK = true
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
		if err := bw.Flush(); err != nil {
			return err
		}
	}
	return sc.Err()
}

// runServe loads espeak, ONNX Runtime, and the selected model once, then
// renders each request to the requested WAV path.
func runServe(o options) error {
	if o.modelPath == "" {
		return fmt.Errorf("--model is required in --serve mode")
	}
	if o.espeakDLL == "" || o.espeakData == "" {
		return fmt.Errorf("--espeak-dll and --espeak-data are required in --serve mode")
	}
	family := models.Family(o.family)
	if err := espeak.Init(o.espeakDLL, o.espeakData); err != nil {
		return err
	}

	ortPath := o.ortPath
	if ortPath == "" {
		return fmt.Errorf("--ort is required in --serve mode")
	}
	ort.SetSharedLibraryPath(ortPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("init onnxruntime: %w", err)
	}
	defer ort.DestroyEnvironment()

	engine, err := models.New(family, o.modelPath, o.configPath, o.voicesPath)
	if err != nil {
		return err
	}
	defer engine.Destroy()

	return serveLoop(o.in, o.out, func(req serveRequest) error {
		if strings.TrimSpace(req.Text) == "" {
			return fmt.Errorf("empty text")
		}
		if req.Voice == "" {
			return fmt.Errorf("missing voice")
		}
		if req.Out == "" {
			return fmt.Errorf("missing out path")
		}
		lang := req.Lang
		if lang == "" {
			lang = "en-us"
		}
		speed := req.Speed
		if speed == 0 {
			speed = 1.0
		}
		ps, err := phonemize(req.Text, lang)
		if err != nil {
			return err
		}
		if strings.TrimSpace(ps) == "" {
			return fmt.Errorf("no phonemes produced")
		}
		audio, err := engine.SynthesizePhonemes(ps, req.Voice, speed)
		if err != nil {
			return err
		}
		return wav.Write(req.Out, audio, engine.SampleRate())
	})
}
