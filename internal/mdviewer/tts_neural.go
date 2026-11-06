package mdviewer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	runtimePkg "runtime"
	"strings"
	"sync"

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

// neuralRequest is one newline-delimited request to the helper's --serve mode.
type neuralRequest struct {
	Text  string  `json:"text"`
	Voice string  `json:"voice"`
	Speed float64 `json:"speed"`
	Lang  string  `json:"lang"`
	Out   string  `json:"out"`
}

type neuralResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// neuralServer keeps one helper process (and its loaded model) alive across
// lines. Requests are serialized because the helper renders one line at a time.
type neuralServer struct {
	family string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	done   chan struct{}
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
			dir: abs,
			exe: exePath,
			ort: ortPath,
			espeakDLL: firstExisting(
				filepath.Join(abs, "espeak-ng.dll"),
				filepath.Join(abs, "libespeak-ng.1.dylib"),
				filepath.Join(abs, "libespeak-ng.dylib"),
				filepath.Join(abs, "..", "piper", "libespeak-ng.1.dylib"),
				filepath.Join(abs, "..", "piper", "libespeak-ng.dylib"),
				filepath.Join(abs, "espeak-ng"),
			),
			// espeak_Initialize wants a directory it can find espeak-ng-data in.
			// The darwin binding accepts either the data directory or its
			// parent; Windows receives the data directory itself.
			espeakData: firstExisting(
				filepath.Join(abs, "espeak-ng-data"),
				filepath.Join(abs, "..", "piper", "espeak-ng-data"),
				filepath.Join(abs, "..", "piper", "engine", "windows", "piper", "espeak-ng-data"),
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

// getNeuralVoices lists Paradee voices via the helper. Kokoro is hidden by
// default: this CPU renders it at roughly 0.2x real time, far too slow for
// line-by-line reading. Set MDVIEWER_NEURAL_KOKORO=1 to list it anyway.
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
	if assets.kokoroModel != "" && os.Getenv("MDVIEWER_NEURAL_KOKORO") == "1" {
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

// neuralServerFor returns the running helper for family, starting it lazily.
// The call must not hold neuralMu.
func (a *App) neuralServerFor(family string, assets *neuralAssets) (*neuralServer, error) {
	a.neuralMu.Lock()
	if srv := a.neuralServers[family]; srv != nil {
		select {
		case <-srv.done:
			delete(a.neuralServers, family)
		default:
			a.neuralMu.Unlock()
			return srv, nil
		}
	}
	a.neuralMu.Unlock()

	args := []string{
		"--serve",
		"--family", family,
		"--model", neuralModel(assets, family),
		"--config", neuralConfig(assets, family),
		"--ort", assets.ort,
		"--espeak-dll", assets.espeakDLL,
		"--espeak-data", assets.espeakData,
	}
	if family == "kokoro" {
		args = append(args, "--voices", assets.kokoroVoices)
	}

	cmd := exec.Command(assets.exe, args...)
	cmd.Dir = assets.dir
	hideProcessWindow(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	srv := &neuralServer{
		family: family,
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		done:   make(chan struct{}),
	}
	go func() {
		_ = cmd.Wait()
		close(srv.done)
	}()

	a.neuralMu.Lock()
	if a.neuralServers == nil {
		a.neuralServers = map[string]*neuralServer{}
	}
	// Another request may have raced us; prefer the existing server.
	if existing := a.neuralServers[family]; existing != nil {
		a.neuralMu.Unlock()
		killProcessTree(cmd)
		return existing, nil
	}
	a.neuralServers[family] = srv
	a.neuralMu.Unlock()
	return srv, nil
}

func neuralModel(assets *neuralAssets, family string) string {
	if family == "kokoro" {
		return assets.kokoroModel
	}
	return assets.paradeeModel
}

func neuralConfig(assets *neuralAssets, family string) string {
	if family == "kokoro" {
		return assets.kokoroConfig
	}
	return assets.paradeeConfig
}

// stop drops the server after a protocol or process failure, so the next line
// starts a fresh one.
func (s *neuralServer) stop() {
	killProcessTree(s.cmd)
	_ = s.stdin.Close()
}

// synth renders one line through the persistent server.
func (s *neuralServer) synth(req neuralRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err := s.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("neural helper write: %w", err)
	}
	line, err := s.stdout.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("neural helper read: %w", err)
	}
	var resp neuralResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("neural helper response: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("neural helper: %s", resp.Error)
	}
	return nil
}

// synthesizeNeuralFile renders one line with the neural helper.
func (a *App) synthesizeNeuralFile(family, voice, text string, rate int, entry *prefetchedAudio) (string, error) {
	assets := a.resolveNeural()
	if assets.exe == "" {
		return "", fmt.Errorf("neural engine not found")
	}
	if neuralModel(assets, family) == "" || neuralConfig(assets, family) == "" {
		return "", fmt.Errorf("neural %s assets not found", family)
	}

	lang := "en-us"
	if strings.HasPrefix(voice, "b") {
		lang = "en-gb"
	}

	tmp := ttsTempFile(".wav")
	srv, err := a.neuralServerFor(family, assets)
	if err != nil {
		return "", err
	}

	// Track the render for Stop: the persistent command stays alive between
	// lines, so Stop kills the whole server rather than this entry.
	if entry != nil {
		a.ttsMutex.Lock()
		entry.cmd = srv.cmd
		a.ttsMutex.Unlock()
	}

	err = srv.synth(neuralRequest{
		Text:  text,
		Voice: voice,
		Speed: tts.NeuralSpeed(rate),
		Lang:  lang,
		Out:   tmp,
	})
	if err != nil {
		a.dropNeuralServer(family, srv)
		os.Remove(tmp)
		return "", err
	}
	if _, err := os.Stat(tmp); err != nil {
		return "", fmt.Errorf("neural engine produced no audio: %w", err)
	}
	return tmp, nil
}

// dropNeuralServer removes srv from the cache if it is still the active one.
func (a *App) dropNeuralServer(family string, srv *neuralServer) {
	a.neuralMu.Lock()
	if a.neuralServers[family] == srv {
		delete(a.neuralServers, family)
	}
	a.neuralMu.Unlock()
	srv.stop()
}

// stopNeuralServers kills every helper process (called by Stop and shutdown).
func (a *App) stopNeuralServers() {
	a.neuralMu.Lock()
	servers := a.neuralServers
	a.neuralServers = map[string]*neuralServer{}
	a.neuralMu.Unlock()
	for _, srv := range servers {
		srv.stop()
	}
}
