//go:build windows

// Package espeak wraps the bundled espeak-ng DLL with just enough API to
// produce IPA phonemes. The DLL is loaded at runtime, so building the helper
// does not require it to be present.
package espeak

/*
#include <stdint.h>

// cstr_len and cstr_copy let Go read espeak's static C string without
// converting a uintptr back to unsafe.Pointer in Go code.
static int cstr_len(uintptr_t src) {
	const char *s = (const char *)src;
	int n = 0;
	while (s[n] != 0) {
		n++;
	}
	return n;
}

static int cstr_copy(uintptr_t src, char *dst, int cap) {
	const char *s = (const char *)src;
	int i = 0;
	while (i < cap-1 && s[i] != 0) {
		dst[i] = s[i];
		i++;
	}
	dst[i] = 0;
	return i;
}
*/
import "C"

import (
	"fmt"
	"sync"
	"syscall"
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

	procInit       *syscall.Proc
	procSetVoice   *syscall.Proc
	procToPhonemes *syscall.Proc
)

// Init loads and initializes espeak-ng. Subsequent calls are no-ops.
func Init(dllPath, dataDir string) error {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return initErr
	}
	loaded = true

	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		initErr = fmt.Errorf("load espeak dll: %w", err)
		return initErr
	}
	pInit, err := dll.FindProc("espeak_Initialize")
	if err != nil {
		initErr = err
		return initErr
	}
	pSet, err := dll.FindProc("espeak_SetVoiceByName")
	if err != nil {
		initErr = err
		return initErr
	}
	pPhon, err := dll.FindProc("espeak_TextToPhonemes")
	if err != nil {
		initErr = err
		return initErr
	}
	procInit, procSetVoice, procToPhonemes = pInit, pSet, pPhon

	dataPtr, err := syscall.BytePtrFromString(dataDir)
	if err != nil {
		initErr = err
		return initErr
	}
	rate, _, err := procInit.Call(
		uintptr(audioOutputRetrieval), 0, uintptr(unsafe.Pointer(dataPtr)), 0)
	if rate == 0 || rate == ^uintptr(0) {
		initErr = fmt.Errorf("espeak_Initialize failed: %v", err)
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

	voicePtr, err := syscall.BytePtrFromString(lang)
	if err != nil {
		return "", err
	}
	if r, _, _ := procSetVoice.Call(uintptr(unsafe.Pointer(voicePtr))); r != 0 {
		return "", fmt.Errorf("espeak_SetVoiceByName(%q) failed", lang)
	}

	textPtr, err := syscall.BytePtrFromString(text)
	if err != nil {
		return "", err
	}
	ptr := unsafe.Pointer(textPtr)

	var out []byte
	for {
		raw, _, _ := procToPhonemes.Call(
			uintptr(unsafe.Pointer(&ptr)), textModeUTF8, phonemesIPA)
		if raw == 0 {
			break
		}
		s := cString(raw)
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

// cString copies a NUL-terminated C string owned by espeak. The pointer stays
// valid until the next espeak call, which we do not make while copying.
func cString(p uintptr) string {
	if p == 0 {
		return ""
	}
	n := int(C.cstr_len(C.uintptr_t(p)))
	buf := make([]byte, n+1)
	C.cstr_copy(C.uintptr_t(p), (*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)))
	return string(buf[:n])
}
