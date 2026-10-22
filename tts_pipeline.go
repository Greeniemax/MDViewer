package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runtimePkg "runtime"
	"strconv"
	"strings"
	"time"

	"MDViewer/internal/tts"
)

// prefetchedAudio is one line being rendered to a temporary audio file ahead
// of playback. The frontend asks for the next line while the current one is
// still being spoken, so playback can start the instant the previous line ends.
type prefetchedAudio struct {
	done    chan struct{}
	path    string
	err     error
	cmd     *exec.Cmd // synthesis process, so Stop can kill an in-flight render
	claimed bool      // a SpeakLine call is waiting for this entry
}

func prefetchKey(text, voice string, rate int) string {
	return strconv.Itoa(rate) + "\x00" + voice + "\x00" + text
}

func ttsTempFile(ext string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("mdviewer_tts_%d%s", time.Now().UnixNano(), ext))
}

// PrefetchLine starts rendering a line in the background and returns
// immediately. Call it for the upcoming line while the current one plays.
// Duplicate requests for the same text/voice/rate are ignored.
func (a *App) PrefetchLine(text, voice string, rate int) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	key := prefetchKey(text, voice, rate)

	a.ttsMutex.Lock()
	if a.stopRequested || a.prefetch == nil {
		a.ttsMutex.Unlock()
		return nil
	}
	if _, exists := a.prefetch[key]; exists {
		a.ttsMutex.Unlock()
		return nil
	}
	entry := &prefetchedAudio{done: make(chan struct{})}
	a.prefetch[key] = entry
	a.ttsMutex.Unlock()

	go a.runPrefetch(key, text, voice, rate, entry)
	return nil
}

func (a *App) runPrefetch(key, text, voice string, rate int, entry *prefetchedAudio) {
	path, err := a.synthesizeToFile(text, voice, rate, entry)

	a.ttsMutex.Lock()
	current, exists := a.prefetch[key]
	if !exists || current != entry {
		// Stop (or a newer session) discarded this entry: throw the audio away.
		a.ttsMutex.Unlock()
		if path != "" {
			os.Remove(path)
		}
		close(entry.done)
		return
	}
	entry.path = path
	entry.err = err
	a.ttsMutex.Unlock()
	close(entry.done)
}

// synthesizeToFile renders text to a complete audio file. It prefers the
// bundled Piper engine and falls back to the OS voice if that fails (unless the
// failure was caused by the user pressing Stop).
func (a *App) synthesizeToFile(text, voice string, rate int, entry *prefetchedAudio) (string, error) {
	if id, ok := tts.PiperVoiceID(voice); ok {
		path, err := a.synthesizePiperFile(text, id, rate, entry)
		if err == nil {
			return path, nil
		}
		a.ttsMutex.Lock()
		stopped := a.stopRequested
		a.ttsMutex.Unlock()
		if stopped {
			return "", err
		}
		// Natural engine failed (not a user Stop) — fall back to the native voice.
		if runtimePkg.GOOS == "windows" {
			return a.synthesizeWindowsFile(text, "", rate, entry)
		}
		return a.synthesizeMacOSFile(text, "", rate, entry)
	}
	if runtimePkg.GOOS == "windows" {
		return a.synthesizeWindowsFile(text, voice, rate, entry)
	}
	return a.synthesizeMacOSFile(text, voice, rate, entry)
}

// runTracked starts cmd, records it on the prefetch entry (if any) so Stop can
// kill it, then waits for it to finish.
func (a *App) runTracked(cmd *exec.Cmd, entry *prefetchedAudio) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	if entry != nil {
		a.ttsMutex.Lock()
		entry.cmd = cmd
		stopped := a.stopRequested
		a.ttsMutex.Unlock()
		if stopped {
			killProcessTree(cmd)
		}
	}
	return cmd.Wait()
}

// synthesizeWindowsFile renders a line with the selected Windows engine.
func (a *App) synthesizeWindowsFile(text, voice string, rate int, entry *prefetchedAudio) (string, error) {
	engine, voiceID := parseWindowsVoice(voice)
	if engine == "onecore" {
		return a.synthesizeWindowsOneCoreFile(text, voiceID, rate, entry)
	}
	return a.synthesizeWindowsSAPIFile(text, voiceID, rate, entry)
}

// synthesizeWindowsSAPIFile renders via legacy SAPI desktop voices
// (System.Speech) into a WAV file.
func (a *App) synthesizeWindowsSAPIFile(text, voiceID string, rate int, entry *prefetchedAudio) (string, error) {
	path := ttsTempFile(".wav")
	escapedText := escapePowerShellSingleQuoted(text)
	escapedPath := escapePowerShellSingleQuoted(path)

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
		psScript += fmt.Sprintf(`
try {
    $synth.SelectVoice('%s')
} catch {
    # If voice selection fails, use default
}
`, escapePowerShellSingleQuoted(voiceID))
	}

	psScript += fmt.Sprintf(`
$synth.SetOutputToWaveFile('%s')
$synth.Speak('%s')
$synth.Dispose()
`, escapedPath, escapedText)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	hideProcessWindow(cmd)

	if err := a.runTracked(cmd, entry); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// synthesizeWindowsOneCoreFile renders via OneCore voices (Settings > Speech)
// into a WAV file.
func (a *App) synthesizeWindowsOneCoreFile(text, voiceID string, rate int, entry *prefetchedAudio) (string, error) {
	path := ttsTempFile(".wav")
	escapedText := escapePowerShellSingleQuoted(text)
	escapedPath := escapePowerShellSingleQuoted(path)
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
		psScript += fmt.Sprintf(`
$selected = [Windows.Media.SpeechSynthesis.SpeechSynthesizer]::AllVoices | Where-Object { $_.Id -eq '%s' } | Select-Object -First 1
if ($selected) { $synth.Voice = $selected }
`, escapePowerShellSingleQuoted(voiceID))
	}

	psScript += fmt.Sprintf(`
$winrtStream = Await ($synth.SynthesizeTextToStreamAsync('%s')) ([Windows.Media.SpeechSynthesis.SpeechSynthesisStream])
$src = [System.IO.WindowsRuntimeStreamExtensions]::AsStreamForRead($winrtStream)
$dst = [System.IO.File]::Create('%s')
$src.CopyTo($dst)
$dst.Dispose()
$src.Dispose()
$winrtStream.Dispose()
$synth.Dispose()
`, escapedText, escapedPath)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	hideProcessWindow(cmd)

	if err := a.runTracked(cmd, entry); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// synthesizeMacOSFile renders a line with macOS say into an AIFF file.
func (a *App) synthesizeMacOSFile(text, voice string, rate int, entry *prefetchedAudio) (string, error) {
	path := ttsTempFile(".aiff")

	var cmd *exec.Cmd
	if voice != "" && voice != "default" {
		cmd = exec.Command("say", "-v", voice, "-r", strconv.Itoa(rate), "-o", path, "--file-format=AIFF", text)
	} else {
		cmd = exec.Command("say", "-r", strconv.Itoa(rate), "-o", path, "--file-format=AIFF", text)
	}

	if err := a.runTracked(cmd, entry); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// playAudioFile plays a pre-rendered WAV/AIFF file to completion. Stop kills the
// tracked player process, which cuts the audio off immediately.
func (a *App) playAudioFile(path string) error {
	var cmd *exec.Cmd
	if runtimePkg.GOOS == "windows" {
		escaped := escapePowerShellSingleQuoted(path)
		ps := fmt.Sprintf("(New-Object System.Media.SoundPlayer '%s').PlaySync()", escaped)
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
		hideProcessWindow(cmd)
	} else {
		cmd = exec.Command("afplay", path)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	a.ttsMutex.Lock()
	a.ttsProcess = cmd
	stopped := a.stopRequested
	a.ttsMutex.Unlock()
	if stopped {
		killProcessTree(cmd)
	}

	err := cmd.Wait()

	a.ttsMutex.Lock()
	if a.ttsProcess == cmd {
		a.ttsProcess = nil
	}
	a.ttsMutex.Unlock()
	return err
}

// killProcessTree terminates cmd and, on Windows, all of its children (the
// audio host / synthesis helpers).
func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtimePkg.GOOS == "windows" {
		killer := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
		hideProcessWindow(killer)
		_ = killer.Run()
		return
	}
	_ = cmd.Process.Kill()
}
