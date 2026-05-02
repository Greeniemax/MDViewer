package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	diskReloadDebounce = 280 * time.Millisecond
	ignoreOwnWriteFor  = 900 * time.Millisecond
)

// SetWatchedFilePaths replaces the set of files monitored for external (on-disk) changes.
func (a *App) SetWatchedFilePaths(paths []string) error {
	need := make(map[string]struct{})
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(filepath.Clean(p))
		if err != nil {
			continue
		}
		need[abs] = struct{}{}
	}

	a.watchMu.Lock()
	defer a.watchMu.Unlock()

	if a.fileWatcher == nil {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return err
		}
		a.fileWatcher = w
		a.watchedAbs = make(map[string]struct{})
		a.reloadTimer = make(map[string]*time.Timer)
		a.lastAppWrite = make(map[string]time.Time)
		go a.watchLoop()
	}

	for p := range a.watchedAbs {
		if _, keep := need[p]; !keep {
			_ = a.fileWatcher.Remove(p)
			delete(a.watchedAbs, p)
			if t := a.reloadTimer[p]; t != nil {
				t.Stop()
				delete(a.reloadTimer, p)
			}
		}
	}

	for p := range need {
		if _, ok := a.watchedAbs[p]; ok {
			continue
		}
		if err := a.fileWatcher.Add(p); err != nil {
			continue
		}
		a.watchedAbs[p] = struct{}{}
	}

	return nil
}

func (a *App) watchLoop() {
	for {
		select {
		case err, ok := <-a.fileWatcher.Errors:
			if !ok {
				return
			}
			if err != nil {
				// Non-fatal; keep watching
				_ = err
			}
		case ev, ok := <-a.fileWatcher.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			if key, ok := a.resolveWatchedKey(ev.Name); ok {
				a.schedulePathReload(key)
			}
		}
	}
}

func (a *App) resolveWatchedKey(name string) (string, bool) {
	clean, err := filepath.Abs(filepath.Clean(name))
	if err != nil {
		clean = filepath.Clean(name)
	}

	a.watchMu.Lock()
	defer a.watchMu.Unlock()

	if _, ok := a.watchedAbs[clean]; ok {
		return clean, true
	}
	for p := range a.watchedAbs {
		if strings.EqualFold(filepath.Clean(p), clean) {
			return p, true
		}
	}
	return "", false
}

func (a *App) schedulePathReload(abs string) {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()

	if _, ok := a.watchedAbs[abs]; !ok {
		return
	}
	if t, ok := a.reloadTimer[abs]; ok {
		t.Stop()
	}
	pathCopy := abs
	a.reloadTimer[abs] = time.AfterFunc(diskReloadDebounce, func() {
		a.finishDebouncedReload(pathCopy)
	})
}

func (a *App) finishDebouncedReload(abs string) {
	a.watchMu.Lock()
	if _, ok := a.watchedAbs[abs]; !ok {
		delete(a.reloadTimer, abs)
		a.watchMu.Unlock()
		return
	}
	if t, ok := a.lastAppWrite[abs]; ok && time.Since(t) < ignoreOwnWriteFor {
		delete(a.reloadTimer, abs)
		a.watchMu.Unlock()
		return
	}
	delete(a.reloadTimer, abs)
	a.watchMu.Unlock()

	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}

	a.watchMu.Lock()
	if _, ok := a.watchedAbs[abs]; !ok {
		a.watchMu.Unlock()
		return
	}
	a.watchMu.Unlock()

	type payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	b, err := json.Marshal(payload{Path: abs, Content: string(data)})
	if err != nil {
		return
	}
	runtime.EventsEmit(a.ctx, "document-disk-changed", string(b))
}

func (a *App) recordOwnDiskWrite(path string) {
	if path == "" {
		return
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		abs = filepath.Clean(path)
	}

	a.watchMu.Lock()
	defer a.watchMu.Unlock()
	if a.lastAppWrite == nil {
		a.lastAppWrite = make(map[string]time.Time)
	}
	a.lastAppWrite[abs] = time.Now()
}
