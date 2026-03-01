package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runtimePkg "runtime"
	"strings"
	"sync"
	"syscall"
	"time"

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
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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
	return os.WriteFile(filepath, []byte(content), 0644)
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
	if runtimePkg.GOOS == "windows" {
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

// speakLineWindows speaks a line using Windows SAPI
func (a *App) speakLineWindows(text string, voice string, rate int) error {
	// Escape text for PowerShell
	escapedText := strings.ReplaceAll(text, "'", "''")
	escapedText = strings.ReplaceAll(escapedText, "`", "``")
	
	// Convert rate (100-250 WPM) to SAPI rate (-10 to 10)
	// 100 WPM -> -5, 175 WPM -> 0, 250 WPM -> 5
	sapiRate := ((rate - 175) * 5) / 75
	if sapiRate < -10 {
		sapiRate = -10
	}
	if sapiRate > 10 {
		sapiRate = 10
	}

	// Build PowerShell script
	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Speech
$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
$synth.Rate = %d
`, sapiRate)

	// Add voice selection if specified
	if voice != "" && voice != "default" {
		psScript += fmt.Sprintf(`
try {
    $synth.SelectVoice('%s')
} catch {
    # If voice selection fails, use default
}
`, voice)
	}

	psScript += fmt.Sprintf(`
$synth.Speak('%s')
$synth.Dispose()
`, escapedText)

	// Execute PowerShell command with hidden window
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	
	// Hide the console window on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	
	a.ttsMutex.Lock()
	a.ttsProcess = cmd
	a.ttsMutex.Unlock()

	return cmd.Run()
}

// Stop stops the current TTS
func (a *App) Stop() error {
	a.ttsMutex.Lock()
	defer a.ttsMutex.Unlock()

	if a.ttsProcess != nil {
		if runtimePkg.GOOS == "windows" {
			// Kill PowerShell processes running SAPI
			exec.Command("taskkill", "/F", "/IM", "powershell.exe", "/FI", "WINDOWTITLE eq Windows PowerShell").Run()
			// Also try to kill by process tree
			if a.ttsProcess.Process != nil {
				a.ttsProcess.Process.Kill()
			}
		} else {
			// Kill the say process on macOS
			exec.Command("pkill", "-f", "say").Run()
			exec.Command("pkill", "-f", "afplay").Run()
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
		if strings.TrimSpace(line) != "" {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				voiceName := parts[0]
				voices = append(voices, voiceName)
			}
		}
	}
	return voices, nil
}

// getAvailableVoicesWindows returns available voices on Windows
func (a *App) getAvailableVoicesWindows() ([]string, error) {
	psScript := `
Add-Type -AssemblyName System.Speech
$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
$synth.GetInstalledVoices() | ForEach-Object { $_.VoiceInfo.Name }
$synth.Dispose()
`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	
	// Hide the console window on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	voices := []string{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			voices = append(voices, line)
		}
	}
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
