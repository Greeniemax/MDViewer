package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runtimePkg "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"MDViewer/internal/tts"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// App struct
type App struct {
	ctx         context.Context
	currentFile string
	content     string
	ttsProcess  *exec.Cmd
	ttsMutex    sync.Mutex
	isSpeaking  bool

	watchMu      sync.Mutex
	fileWatcher  *fsnotify.Watcher
	watchedAbs   map[string]struct{}
	reloadTimer  map[string]*time.Timer
	lastAppWrite map[string]time.Time
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startupFileFromArgs returns the path passed when Windows opens a file
// (e.g. double-click .md) or the user drops a file onto the executable.
func startupFileFromArgs() string {
	if len(os.Args) < 2 {
		return ""
	}
	path := strings.TrimSpace(os.Args[1])
	if path == "" || strings.HasPrefix(path, "-") {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return filepath.Clean(path)
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if path := startupFileFromArgs(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			a.currentFile = path
			a.content = string(data)
		}
	}
}

// OpenFileDialog opens a native file picker for markdown files
func (a *App) OpenFileDialog() (string, error) {
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Markdown File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Markdown Files (*.md)",
				Pattern:     "*.md;*.markdown",
			},
			{
				DisplayName: "Text Files (*.txt)",
				Pattern:     "*.txt",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	a.currentFile = filePath
	a.content = string(content)

	return filePath, nil
}

// SaveFileDialog opens a native save dialog
func (a *App) SaveFileDialog(content string) (string, error) {
	defaultFilename := "untitled.md"
	if a.currentFile != "" {
		defaultFilename = filepath.Base(a.currentFile)
	}

	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Markdown File",
		DefaultFilename: defaultFilename,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Markdown Files (*.md)",
				Pattern:     "*.md",
			},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil
	}

	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", err
	}

	a.recordOwnDiskWrite(filePath)
	a.currentFile = filePath
	a.content = content

	return filePath, nil
}

// ReadFile reads a markdown file and returns its content
func (a *App) ReadFile(filepath string) (string, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	a.currentFile = filepath
	a.content = string(content)
	return string(content), nil
}

// SaveFile saves content to a file
func (a *App) SaveFile(filepath, content string) error {
	a.currentFile = filepath
	a.content = content
	err := os.WriteFile(filepath, []byte(content), 0644)
	if err != nil {
		return err
	}
	a.recordOwnDiskWrite(filepath)
	return nil
}

// SetActiveDocument syncs which document is active in the backend (used for save dialogs and helpers).
func (a *App) SetActiveDocument(path string, documentContent string) {
	a.currentFile = path
	a.content = documentContent
}

// GetCurrentFile returns the current file path
func (a *App) GetCurrentFile() string {
	return a.currentFile
}

// GetContent returns the current content
func (a *App) GetContent() string {
	return a.content
}

// SpeakLine speaks a single line and waits for completion
func (a *App) SpeakLine(text string, voice string, rate int) error {
	a.ttsMutex.Lock()

	// Check if empty line
	if strings.TrimSpace(text) == "" {
		a.ttsMutex.Unlock()
		return nil // Skip empty lines silently
	}

	a.isSpeaking = true
	a.ttsMutex.Unlock()

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

	// Update state after completion
	a.ttsMutex.Lock()
	a.isSpeaking = false
	a.ttsProcess = nil
	a.ttsMutex.Unlock()

	return err
}

// speakLineMacOS speaks a line using macOS say command
func (a *App) speakLineMacOS(text string, voice string, rate int) error {
	// Create temp file for audio output to ensure complete processing
	tmpFile := fmt.Sprintf("/tmp/mdviewer_tts_%d.aiff", time.Now().UnixNano())

	var cmd *exec.Cmd
	if voice != "" && voice != "default" {
		cmd = exec.Command("say", "-v", voice, "-r", fmt.Sprintf("%d", rate), "-o", tmpFile, "--file-format=AIFF", text)
	} else {
		cmd = exec.Command("say", "-r", fmt.Sprintf("%d", rate), "-o", tmpFile, "--file-format=AIFF", text)
	}

	a.ttsMutex.Lock()
	a.ttsProcess = cmd
	a.ttsMutex.Unlock()

	// Run and wait for file creation
	err := cmd.Run()
	
	if err != nil {
		return err
	}

	// Now play the complete audio file
	playCmd := exec.Command("afplay", tmpFile)
	
	a.ttsMutex.Lock()
	a.ttsProcess = playCmd
	a.ttsMutex.Unlock()
	
	err = playCmd.Run()

	// Clean up temp file
	os.Remove(tmpFile)

	return err
}

// escapePowerShellSingleQuoted escapes text for use inside PowerShell single-quoted strings.
func escapePowerShellSingleQuoted(text string) string {
	text = strings.ReplaceAll(text, "'", "''")
	return strings.ReplaceAll(text, "`", "``")
}

// parseWindowsVoice splits "engine\tid" voice selectors from the UI.
// Legacy plain SAPI voice names are still accepted.
func parseWindowsVoice(voice string) (engine, id string) {
	if voice == "" || voice == "default" {
		return "sapi", ""
	}
	parts := strings.SplitN(voice, "\t", 2)
	if len(parts) == 2 && (parts[0] == "sapi" || parts[0] == "onecore") {
		return parts[0], parts[1]
	}
	return "sapi", voice
}

// wpmToOneCoreRate maps the UI speech rate (100-250 WPM) to OneCore SpeakingRate (0.5-2.0).
func wpmToOneCoreRate(rate int) float64 {
	speakingRate := 0.65 + float64(rate-100)*0.9/150.0
	if speakingRate < 0.5 {
		return 0.5
	}
	if speakingRate > 2.0 {
		return 2.0
	}
	return speakingRate
}

func (a *App) runHiddenPowerShell(script string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	hideProcessWindow(cmd)

	a.ttsMutex.Lock()
	a.ttsProcess = cmd
	a.ttsMutex.Unlock()

	return cmd.Run()
}

// speakLineWindows speaks a line using Windows SAPI or OneCore voices.
func (a *App) speakLineWindows(text string, voice string, rate int) error {
	engine, voiceID := parseWindowsVoice(voice)
	if engine == "onecore" {
		return a.speakLineWindowsOneCore(text, voiceID, rate)
	}
	return a.speakLineWindowsSAPI(text, voiceID, rate)
}

// speakLineWindowsSAPI speaks using legacy SAPI desktop voices (System.Speech).
func (a *App) speakLineWindowsSAPI(text, voiceID string, rate int) error {
	escapedText := escapePowerShellSingleQuoted(text)

	sapiRate := ((rate - 175) * 5) / 75
	if sapiRate < -10 {
		sapiRate = -10
	}
	if sapiRate > 10 {
		sapiRate = 10
	}

	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Speech
$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
$synth.Rate = %d
`, sapiRate)

	if voiceID != "" {
		escapedVoice := escapePowerShellSingleQuoted(voiceID)
		psScript += fmt.Sprintf(`
try {
    $synth.SelectVoice('%s')
} catch {
    # If voice selection fails, use default
}
`, escapedVoice)
	}

	psScript += fmt.Sprintf(`
$synth.Speak('%s')
$synth.Dispose()
`, escapedText)

	return a.runHiddenPowerShell(psScript)
}

// speakLineWindowsOneCore speaks using Windows OneCore voices (Settings > Speech / language packs).
func (a *App) speakLineWindowsOneCore(text, voiceID string, rate int) error {
	escapedText := escapePowerShellSingleQuoted(text)
	speakingRate := wpmToOneCoreRate(rate)

	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Runtime.WindowsRuntime
[Windows.Foundation.IAsyncOperation`+"`"+`1, Windows.Foundation, ContentType=WindowsRuntime] | Out-Null
[Windows.Media.SpeechSynthesis.SpeechSynthesizer, Windows.Media.SpeechSynthesis, ContentType=WindowsRuntime] | Out-Null
[Windows.Media.SpeechSynthesis.SpeechSynthesisStream, Windows.Media.SpeechSynthesis, ContentType=WindowsRuntime] | Out-Null
$_taskMethods = [System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
    $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1
}
$asTaskGeneric = ($_taskMethods | Where-Object { $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`+"`"+`1' })[0]
Function Await($WinRtTask, $ResultType) {
    $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
    $netTask = $asTask.Invoke($null, @($WinRtTask))
    $netTask.Wait(-1) | Out-Null
    $netTask.Result
}
$synth = New-Object Windows.Media.SpeechSynthesis.SpeechSynthesizer
$synth.Options.SpeakingRate = %g
`, speakingRate)

	if voiceID != "" {
		escapedVoice := escapePowerShellSingleQuoted(voiceID)
		psScript += fmt.Sprintf(`
$selected = [Windows.Media.SpeechSynthesis.SpeechSynthesizer]::AllVoices | Where-Object { $_.Id -eq '%s' } | Select-Object -First 1
if ($selected) { $synth.Voice = $selected }
`, escapedVoice)
	}

	psScript += fmt.Sprintf(`
$winrtStream = Await ($synth.SynthesizeTextToStreamAsync('%s')) ([Windows.Media.SpeechSynthesis.SpeechSynthesisStream])
$stream = [System.IO.WindowsRuntimeStreamExtensions]::AsStreamForRead($winrtStream)
$player = New-Object System.Media.SoundPlayer $stream
$player.PlaySync()
$player.Dispose()
$stream.Dispose()
$winrtStream.Dispose()
$synth.Dispose()
`, escapedText)

	return a.runHiddenPowerShell(psScript)
}

// Stop stops the current TTS
func (a *App) Stop() error {
	a.ttsMutex.Lock()
	defer a.ttsMutex.Unlock()

	if a.ttsProcess != nil {
		if runtimePkg.GOOS == "windows" {
			// Kill PowerShell processes running SAPI
			exec.Command("taskkill", "/F", "/IM", "powershell.exe", "/FI", "WINDOWTITLE eq Windows PowerShell").Run()
			exec.Command("taskkill", "/F", "/IM", "piper.exe").Run()
			// Also try to kill by process tree
			if a.ttsProcess.Process != nil {
				a.ttsProcess.Process.Kill()
			}
		} else {
			// Kill the say process on macOS
			exec.Command("pkill", "-f", "say").Run()
			exec.Command("pkill", "-f", "afplay").Run()
			exec.Command("pkill", "-f", "piper").Run()
		}
		a.ttsProcess = nil
		a.isSpeaking = false
	}
	return nil
}

// IsSpeaking returns whether TTS is currently active
func (a *App) IsSpeaking() bool {
	a.ttsMutex.Lock()
	defer a.ttsMutex.Unlock()
	return a.isSpeaking
}

// GetAvailableVoices returns list of available TTS voices
func (a *App) GetAvailableVoices() ([]string, error) {
	if runtimePkg.GOOS == "windows" {
		return a.getAvailableVoicesWindows()
	}
	return a.getAvailableVoicesMacOS()
}

// getAvailableVoicesMacOS returns available voices on macOS
func (a *App) getAvailableVoicesMacOS() ([]string, error) {
	cmd := exec.Command("say", "-v", "?")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	voices := []string{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		voiceName := parts[0]
		// Build description from remaining fields (typically: locale # sample)
		desc := ""
		if len(parts) > 1 {
			// Skip the voice name, include the rest
			desc = strings.Join(parts[1:], " ")
			// Trim any leading "#" from sample text
			desc = strings.TrimSpace(desc)
		}
		if desc != "" {
			voices = append(voices, voiceName+"|"+desc)
		} else {
			voices = append(voices, voiceName)
		}
	}
	return voices, nil
}

// getAvailableVoicesWindows returns SAPI desktop voices and OneCore voices installed via Windows Settings.
// Each entry uses "engine\tvoiceId\tlabel" so the UI can show a friendly name and select the right engine.
func (a *App) getAvailableVoicesWindows() ([]string, error) {
	psScript := "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)\n" +
		"$tab = [char]9\n" +
		"$results = New-Object System.Collections.Generic.List[string]\n" +
		"try {\n" +
		"    Add-Type -AssemblyName System.Speech\n" +
		"    $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer\n" +
		"    $synth.GetInstalledVoices() | ForEach-Object {\n" +
		"        $info = $_.VoiceInfo\n" +
		"        $label = if ($info.Description) { $info.Description } else { $info.Name }\n" +
		"        $results.Add(('sapi' + $tab + $info.Name + $tab + $label))\n" +
		"    }\n" +
		"    $synth.Dispose()\n" +
		"} catch {}\n" +
		"try {\n" +
		"    Add-Type -AssemblyName System.Runtime.WindowsRuntime\n" +
		"    [Windows.Foundation.IAsyncOperation`1, Windows.Foundation, ContentType=WindowsRuntime] | Out-Null\n" +
		"    [Windows.Media.SpeechSynthesis.SpeechSynthesizer, Windows.Media.SpeechSynthesis, ContentType=WindowsRuntime] | Out-Null\n" +
		"    [Windows.Media.SpeechSynthesis.SpeechSynthesizer]::AllVoices | ForEach-Object {\n" +
		"        $label = ('{0} - {1}' -f $_.DisplayName, $_.Language)\n" +
		"        $results.Add(('onecore' + $tab + $_.Id + $tab + $label))\n" +
		"    }\n" +
		"} catch {}\n" +
		"$results | ForEach-Object { $_ }\n"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	hideProcessWindow(cmd)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	voices := []string{}
	seen := map[string]struct{}{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		voices = append(voices, line)
	}
	sort.Slice(voices, func(i, j int) bool {
		labelI := voices[i]
		labelJ := voices[j]
		if parts := strings.SplitN(voices[i], "\t", 3); len(parts) == 3 {
			labelI = parts[2]
		}
		if parts := strings.SplitN(voices[j], "\t", 3); len(parts) == 3 {
			labelJ = parts[2]
		}
		return strings.ToLower(labelI) < strings.ToLower(labelJ)
	})
	return voices, nil
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "MDViewer",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 30, G: 38, B: 46, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
