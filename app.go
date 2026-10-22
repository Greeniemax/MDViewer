package main

import (
	"context"
	"embed"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	runtimePkg "runtime"
	"sort"
	"strings"
	"sync"
	"time"

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
	ctx           context.Context
	currentFile   string
	content       string
	ttsProcess    *exec.Cmd
	ttsMutex      sync.Mutex
	isSpeaking    bool
	stopRequested bool
	prefetch      map[string]*prefetchedAudio

	watchMu      sync.Mutex
	fileWatcher  *fsnotify.Watcher
	watchedAbs   map[string]struct{}
	reloadTimer  map[string]*time.Timer
	lastAppWrite map[string]time.Time

	voicesMu         sync.Mutex
	voicesRefreshing bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		prefetch: make(map[string]*prefetchedAudio),
	}
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

// StartSpeaking begins a new reading session: it clears the stop flag and
// discards any audio that was prefetched for a previous session.
func (a *App) StartSpeaking() {
	a.ttsMutex.Lock()
	a.stopRequested = false
	paths := make([]string, 0, len(a.prefetch))
	for _, entry := range a.prefetch {
		if entry.path != "" {
			paths = append(paths, entry.path)
		}
	}
	a.prefetch = make(map[string]*prefetchedAudio)
	a.ttsMutex.Unlock()

	for _, path := range paths {
		os.Remove(path)
	}
}

// SpeakLine speaks a single line and waits for completion. If the same line
// was prefetched (see PrefetchLine), playback starts immediately because the
// audio file is already rendered.
func (a *App) SpeakLine(text string, voice string, rate int) error {
	if strings.TrimSpace(text) == "" {
		return nil // Skip empty lines silently
	}

	a.ttsMutex.Lock()
	a.isSpeaking = true
	a.stopRequested = false

	key := prefetchKey(text, voice, rate)
	entry, prefetched := a.prefetch[key]
	if prefetched {
		entry.claimed = true
	}
	a.ttsMutex.Unlock()

	var path string
	var err error
	if prefetched {
		// Wait for the background render; in the normal flow it is already done
		// because it ran while the previous line was playing.
		<-entry.done
		path, err = entry.path, entry.err

		a.ttsMutex.Lock()
		if a.prefetch[key] == entry {
			delete(a.prefetch, key)
		}
		a.ttsMutex.Unlock()
	} else {
		path, err = a.synthesizeToFile(text, voice, rate, nil)
	}

	a.ttsMutex.Lock()
	stopped := a.stopRequested
	a.ttsMutex.Unlock()
	if stopped {
		if path != "" {
			os.Remove(path)
		}
		a.finishSpeaking()
		return nil
	}

	if err == nil && path != "" {
		err = a.playAudioFile(path)
	}
	if path != "" {
		os.Remove(path)
	}

	a.finishSpeaking()
	return err
}

func (a *App) finishSpeaking() {
	a.ttsMutex.Lock()
	a.isSpeaking = false
	a.ttsProcess = nil
	a.ttsMutex.Unlock()
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

// Stop stops the current TTS immediately: it kills the audio player and any
// in-flight background synthesis, and drops all prefetched audio.
func (a *App) Stop() error {
	a.ttsMutex.Lock()
	a.stopRequested = true
	player := a.ttsProcess
	a.ttsProcess = nil
	a.isSpeaking = false

	prefetch := a.prefetch
	a.prefetch = make(map[string]*prefetchedAudio)

	synths := make([]*exec.Cmd, 0, len(prefetch))
	paths := make([]string, 0, len(prefetch))
	for _, entry := range prefetch {
		if entry.cmd != nil {
			synths = append(synths, entry.cmd)
		}
		if entry.path != "" {
			paths = append(paths, entry.path)
		}
	}
	a.ttsMutex.Unlock()

	// Kill the active player first so audio goes silent right away.
	killProcessTree(player)
	for _, cmd := range synths {
		killProcessTree(cmd)
	}
	for _, path := range paths {
		os.Remove(path)
	}

	if runtimePkg.GOOS != "windows" {
		// Sweep players/synthesizers that may have outlived their parent.
		exec.Command("pkill", "-f", "afplay").Run()
		exec.Command("pkill", "-f", "say").Run()
		exec.Command("pkill", "-f", "piper").Run()
	}
	return nil
}

// IsSpeaking returns whether TTS is currently active
func (a *App) IsSpeaking() bool {
	a.ttsMutex.Lock()
	defer a.ttsMutex.Unlock()
	return a.isSpeaking
}

// GetAvailableVoices returns list of available TTS voices. Native voice
// enumeration is expensive on Windows (it starts PowerShell), so the last
// result is cached and returned immediately. A background refresh keeps the
// cache current and notifies the UI if the list changed.
func (a *App) GetAvailableVoices() ([]string, error) {
	piper := a.getPiperVoices()

	native, cached := loadNativeVoicesCache()
	if cached {
		a.refreshNativeVoicesAsync()
	} else {
		var err error
		native, err = a.queryNativeVoices()
		if err != nil && len(piper) == 0 {
			return nil, err
		}
		if err == nil {
			saveNativeVoicesCache(native)
		}
	}

	return append(piper, native...), nil
}

func (a *App) queryNativeVoices() ([]string, error) {
	if runtimePkg.GOOS == "windows" {
		return a.getAvailableVoicesWindows()
	}
	return a.getAvailableVoicesMacOS()
}

func (a *App) refreshNativeVoicesAsync() {
	a.voicesMu.Lock()
	if a.voicesRefreshing {
		a.voicesMu.Unlock()
		return
	}
	a.voicesRefreshing = true
	a.voicesMu.Unlock()

	go func() {
		defer func() {
			a.voicesMu.Lock()
			a.voicesRefreshing = false
			a.voicesMu.Unlock()
		}()

		native, err := a.queryNativeVoices()
		if err != nil {
			return
		}
		if err := saveNativeVoicesCache(native); err != nil {
			return
		}
		voices := append(a.getPiperVoices(), native...)
		runtime.EventsEmit(a.ctx, "voices-updated", voices)
	}()
}

func nativeVoicesCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "MDViewer", "native_voices.json")
}

func loadNativeVoicesCache() ([]string, bool) {
	path := nativeVoicesCachePath()
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var voices []string
	if err := json.Unmarshal(data, &voices); err != nil || voices == nil {
		return nil, false
	}
	return voices, true
}

func saveNativeVoicesCache(voices []string) error {
	path := nativeVoicesCachePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(voices)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// getAvailableVoicesMacOS returns the voices installed on this Mac (`say -v ?`).
// The output format is "<display name>  <locale>  # <sample sentence>", where
// the display name may contain spaces and parentheses ("Eddy (German (Germany))").
// Only a short list of standard English voices is returned.
func (a *App) getAvailableVoicesMacOS() ([]string, error) {
	cmd := exec.Command("say", "-v", "?")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	type entry struct{ name, locale string }
	var installed []entry
	seen := map[string]struct{}{}
	for _, line := range strings.Split(string(output), "\n") {
		// Drop the "# ..." sample sentence first; it also contains spaces.
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// The locale is the right-most whitespace-separated token.
		cut := strings.LastIndexAny(line, " \t")
		if cut <= 0 {
			continue
		}
		name := strings.TrimSpace(line[:cut])
		locale := strings.TrimSpace(line[cut+1:])
		if name == "" || !strings.HasPrefix(strings.ToLower(locale), "en") {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		installed = append(installed, entry{name: name, locale: locale})
	}

	voices := []string{}
	added := map[string]struct{}{}
	add := func(e entry) {
		if _, exists := added[e.name]; exists {
			return
		}
		added[e.name] = struct{}{}
		voices = append(voices, e.name+"|"+e.locale)
	}

	for _, name := range preferredEnglishVoiceNames {
		for _, e := range installed {
			if strings.EqualFold(e.name, name) {
				add(e)
				break
			}
		}
	}
	if len(voices) == 0 {
		for _, e := range installed {
			if strings.Contains(e.name, "(") || noveltyVoiceNames[e.name] {
				continue
			}
			add(e)
		}
	}
	if len(voices) > maxNativeVoices {
		voices = voices[:maxNativeVoices]
	}
	return voices, nil
}

// Standard English voices in rough order of preference; only entries installed
// on this machine are used.
var preferredEnglishVoiceNames = []string{
	"Samantha", "Alex", "Ava", "Allison", "Susan", "Tom", "Kathy",
	"Daniel", "Serena", "Kate", "Oliver", "Arthur", "Martha",
	"Karen", "Lee", "Matilda", "Moira", "Tessa",
	"Rishi", "Aman", "Tara", "Veena",
}

// Joke/novelty voices that macOS also exposes.
var noveltyVoiceNames = map[string]bool{
	"Albert": true, "Bad News": true, "Bahh": true, "Bells": true,
	"Boing": true, "Bubbles": true, "Cellos": true, "Fred": true,
	"Good News": true, "Grandma": true, "Grandpa": true, "Jester": true,
	"Junior": true, "Organ": true, "Ralph": true, "Rocko": true,
	"Sandy": true, "Shelley": true, "Superstar": true, "Trinoids": true,
	"Whisper": true, "Wobble": true, "Zarvox": true, "Eddy": true,
	"Flo": true, "Reed": true,
}

// getAvailableVoicesWindows returns SAPI desktop voices and OneCore voices installed via Windows Settings.
// Each entry uses "engine\tvoiceId\tlabel" so the UI can show a friendly name and select the right engine.
// PowerShell is the primary enumerator; the registry fallback below guarantees
// the installed voices are listed even when the WinRT/SAPI projections fail.
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

	voices := []string{}
	seen := map[string]struct{}{}
	addVoice := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}
		key := line
		if parts := strings.SplitN(line, "\t", 3); len(parts) == 3 {
			// PowerShell labels and registry labels can differ for the same
			// voice, so de-duplicate on engine + id.
			key = parts[0] + "\t" + parts[1]
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		voices = append(voices, line)
	}

	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			addVoice(line)
		}
	}

	// Installed voices straight from the registry (SAPI + OneCore, user + machine).
	for _, line := range registryVoices() {
		addVoice(line)
	}

	if len(voices) == 0 && err != nil {
		return nil, err
	}

	// Keep the list short and English-only for normal use. If nothing matches
	// (e.g. an unusual localized install), keep what we found instead of
	// returning an empty picker.
	if english := englishNativeVoices(voices); len(english) > 0 {
		voices = english
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
	if len(voices) > maxNativeVoices {
		voices = voices[:maxNativeVoices]
	}
	return voices, nil
}

// maxNativeVoices caps the native list in the voice picker.
const maxNativeVoices = 12

// englishNativeVoices keeps English voices and collapses duplicates that
// Windows exposes through both SAPI and OneCore (same display name).
func englishNativeVoices(voices []string) []string {
	out := make([]string, 0, len(voices))
	seen := map[string]struct{}{}
	for _, line := range voices {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		if !isEnglishNativeVoice(parts[1], parts[2]) {
			continue
		}
		key := voiceBaseName(parts[2])
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, line)
	}
	return out
}

func isEnglishNativeVoice(id, label string) bool {
	id = strings.ToLower(id)
	label = strings.ToLower(label)
	if strings.Contains(label, "english") {
		return true
	}
	for _, tag := range []string{
		"en-us", "en_us", "enus", "en-gb", "en_gb", "engb",
		"en-au", "en_au", "enau", "en-ca", "en_ca", "enca",
		"en-ie", "en_ie", "enie", "en-in", "en_in", "enin",
		"en-nz", "en_nz", "ennz", "en-za", "en_za", "enza",
	} {
		if strings.Contains(id, tag) {
			return true
		}
	}
	return false
}

// voiceBaseName is "Microsoft David Desktop - English (United States)" ->
// "microsoft david", so SAPI and OneCore copies of one voice collapse.
func voiceBaseName(label string) string {
	base := label
	if i := strings.Index(base, " - "); i > 0 {
		base = base[:i]
	}
	base = strings.ReplaceAll(base, " Desktop", "")
	base = strings.ReplaceAll(base, " Mobile", "")
	return strings.ToLower(strings.TrimSpace(base))
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
