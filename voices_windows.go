//go:build windows

package main

import (
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// registryVoices enumerates the speech voices installed on this machine
// straight from the registry. SAPI tokens live under Speech\Voices\Tokens and
// the voices installed from Settings (OneCore) under Speech_OneCore\Voices.
// Each entry uses the same "engine\tid\tlabel" format as the PowerShell scan so
// the UI and the synthesis code can treat both sources identically.
func registryVoices() []string {
	type root struct {
		engine string
		path   string
		view   uint32
	}
	roots := []root{
		{"sapi", `SOFTWARE\Microsoft\Speech\Voices\Tokens`, registry.WOW64_64KEY},
		{"sapi", `SOFTWARE\Microsoft\Speech\Voices\Tokens`, registry.WOW64_32KEY},
		{"onecore", `SOFTWARE\Microsoft\Speech_OneCore\Voices\Tokens`, registry.WOW64_64KEY},
	}
	hives := []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER}

	var out []string
	seen := map[string]struct{}{}

	for _, r := range roots {
		for _, hive := range hives {
			key, err := registry.OpenKey(hive, r.path, registry.READ|r.view)
			if err != nil {
				continue
			}
			names, err := key.ReadSubKeyNames(-1)
			key.Close()
			if err != nil {
				continue
			}

			for _, name := range names {
				sub, err := registry.OpenKey(hive, r.path+`\`+name, registry.READ|r.view)
				if err != nil {
					continue
				}
				label, _, _ := sub.GetStringValue("")
				sub.Close()

				name = strings.TrimSpace(name)
				label = strings.TrimSpace(label)
				if name == "" {
					continue
				}
				if label == "" {
					label = name
				}

				id := r.engine + "\t" + name
				if _, exists := seen[id]; exists {
					continue
				}
				seen[id] = struct{}{}
				out = append(out, r.engine+"\t"+name+"\t"+label)
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}
