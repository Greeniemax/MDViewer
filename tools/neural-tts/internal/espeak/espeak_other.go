//go:build !windows && !darwin

package espeak

import "fmt"

func Init(dllPath, dataDir string) error {
	return fmt.Errorf("espeak binding is not implemented on this platform yet")
}

func Phonemize(text, lang string) (string, error) {
	return "", fmt.Errorf("espeak binding is not implemented on this platform yet")
}
