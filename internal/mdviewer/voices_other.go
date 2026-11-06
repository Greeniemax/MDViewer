//go:build !windows

package mdviewer

// registryVoices is Windows-only; other platforms enumerate their system
// voices with `say -v ?`.
func registryVoices() []string { return nil }
