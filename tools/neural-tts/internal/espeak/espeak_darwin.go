//go:build darwin

// Package espeak wraps the Piper-bundled libespeak-ng with just enough API to
// produce IPA phonemes. The dylib is loaded at runtime, so building the helper
// does not require it to be present.
package espeak

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <stdint.h>

typedef int (*espeak_init_fn)(int, int, const char *, int);
typedef int (*espeak_set_voice_fn)(const char *);
typedef const char *(*espeak_phonemes_fn)(void **, int, int);

static espeak_init_fn g_init;
static espeak_set_voice_fn g_set_voice;
static espeak_phonemes_fn g_phonemes;

// espeak_bridge_load resolves the three entry points we call. It returns 0 on
// failure and leaves the last dlerror() available to the caller.
static int espeak_bridge_load(const char *lib) {
	void *h = dlopen(lib, RTLD_NOW | RTLD_LOCAL);
	if (h == NULL) {
		return 0;
	}
	g_init = (espeak_init_fn)dlsym(h, "espeak_Initialize");
	g_set_voice = (espeak_set_voice_fn)dlsym(h, "espeak_SetVoiceByName");
	g_phonemes = (espeak_phonemes_fn)dlsym(h, "espeak_TextToPhonemes");
	return g_init != NULL && g_set_voice != NULL && g_phonemes != NULL;
}

static const char *espeak_bridge_error(void) { return dlerror(); }

static int espeak_bridge_init(int mode, int buflen, const char *path, int options) {
	return g_init(mode, buflen, path, options);
}

static int espeak_bridge_set_voice(const char *name) {
	return g_set_voice(name);
}

static const char *espeak_bridge_phonemes(void **ptr, int textmode, int phonememode) {
	return g_phonemes(ptr, textmode, phonememode);
}
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

const (
	audioOutputRetrieval = 1
	textModeUTF8         = 1
	phonemesIPA          = 0x02
)

var (
	mu      sync.Mutex
	loaded  bool
	initOK  bool
	initErr error
)

// Init loads and initializes espeak-ng. dllPath may be the libespeak-ng dylib
// itself or any file in its directory; dataDir is the directory that contains
// espeak-ng-data (the Piper bundle root). Subsequent calls are no-ops.
func Init(dllPath, dataDir string) error {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return initErr
	}
	loaded = true

	libPath, err := resolveDylib(dllPath)
	if err != nil {
		initErr = err
		return initErr
	}
	cLib := C.CString(libPath)
	defer C.free(unsafe.Pointer(cLib))
	if C.espeak_bridge_load(cLib) == 0 {
		msg := ""
		if e := C.espeak_bridge_error(); e != nil {
			msg = C.GoString(e)
		}
		initErr = fmt.Errorf("load espeak dylib %s: %s", libPath, msg)
		return initErr
	}

	root := dataRoot(dataDir)
	cRoot := C.CString(root)
	defer C.free(unsafe.Pointer(cRoot))

	rate := C.espeak_bridge_init(audioOutputRetrieval, 0, cRoot, 0)
	if rate <= 0 {
		initErr = fmt.Errorf("espeak_Initialize failed for data dir %s", root)
		return initErr
	}
	initOK = true
	return nil
}

// Phonemize returns the IPA phonemes for one clause. Punctuation is not
// included; callers re-attach it.
func Phonemize(text, lang string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if !initOK {
		return "", fmt.Errorf("espeak not initialized")
	}

	if err := selectVoice(lang); err != nil {
		return "", err
	}

	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	ptr := cText

	var out []byte
	for {
		raw := C.espeak_bridge_phonemes(
			(*unsafe.Pointer)(unsafe.Pointer(&ptr)),
			C.int(textModeUTF8),
			C.int(phonemesIPA),
		)
		if raw == nil {
			break
		}
		s := C.GoString(raw)
		if s == "" {
			break
		}
		if len(out) > 0 {
			out = append(out, ' ')
		}
		out = append(out, s...)
	}
	return string(out), nil
}

// selectVoice sets the espeak voice. The Piper data set exposes language tags
// for en-us but not for en-gb (SetVoiceByName("en-gb") returns EE_NOT_FOUND),
// so British English falls back to the RP voice name that ships with it.
func selectVoice(lang string) error {
	var candidates []string
	switch lang {
	case "en-gb", "en_GB", "en-GB":
		candidates = []string{"en-gb-x-rp", "English (Great Britain)", "en-gb", "en"}
	case "en-us", "en_US", "en-US", "":
		candidates = []string{"en-us"}
	default:
		candidates = []string{lang}
	}
	for _, name := range candidates {
		cName := C.CString(name)
		rc := C.espeak_bridge_set_voice(cName)
		C.free(unsafe.Pointer(cName))
		if rc == 0 {
			return nil
		}
	}
	return fmt.Errorf("espeak_SetVoiceByName(%q) failed", lang)
}

// resolveDylib accepts the dylib path, the espeak-ng binary path, or a
// directory, and returns the libespeak-ng dylib inside it.
func resolveDylib(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("espeak library path is empty")
	}
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() && strings.HasSuffix(path, ".dylib") {
		return path, nil
	}
	dir := path
	if err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}
	for _, name := range []string{"libespeak-ng.1.dylib", "libespeak-ng.dylib"} {
		candidate := filepath.Join(dir, name)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("libespeak-ng dylib not found next to %s", path)
}

// dataRoot returns the directory espeak_Initialize expects: the parent of an
// espeak-ng-data directory (the Piper bundle root). It also accepts that data
// directory directly.
func dataRoot(dataDir string) string {
	if dataDir == "" {
		return dataDir
	}
	if filepath.Base(dataDir) == "espeak-ng-data" {
		return filepath.Dir(dataDir)
	}
	return dataDir
}
