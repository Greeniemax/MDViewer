// Package misaki converts eSpeak NG IPA into the phoneme alphabet Kokoro and
// Paradee were trained on.
//
// The rules are a Go port of web/misaki.js from
// github.com/sahilmahendrakar/paradee (Apache-2.0), itself derived from
// misaki/espeak.py (Apache-2.0). Keep the replacement order: it matters.
package misaki

import (
	"strings"
	"unicode"
)

const boundary = " \t\r\n;:,.!?—…\"“”()"

var replacements = []struct{ from, to string }{
	{"ʔˌn̩", "ʔn"}, {"ʔn̩", "ʔn"},
	{"aɪ", "I"}, {"aʊ", "W"},
	{"dʒ", "ʤ"},
	{"eɪ", "A"}, {"e", "A"},
	{"tʃ", "ʧ"},
	{"ɔɪ", "Y"},
	{"ʲo", "jo"}, {"ʲə", "jə"}, {"ʲ", ""},
	{"ɚ", "əɹ"},
	{"r", "ɹ"},
	{"x", "k"}, {"ç", "k"},
	{"ɬ", "l"},
	{"̃", ""},
}

// ToMisaki normalizes one eSpeak phoneme string. british selects the rules for
// the bf_/bm_ voices.
func ToMisaki(ps string, british bool) string {
	if british {
		ps = strings.ReplaceAll(ps, "eə", "ɛː")
	}
	for _, r := range replacements {
		ps = strings.ReplaceAll(ps, r.from, r.to)
	}
	ps = syllabicConsonants(ps)
	ps = finalSyllabicL(ps)
	ps = replaceOpenA(ps, british)

	if british {
		ps = strings.ReplaceAll(ps, "iə", "ɪə")
		ps = strings.ReplaceAll(ps, "əʊ", "Q")
		ps = theBeforeVowel(ps)
	} else {
		ps = strings.ReplaceAll(ps, "oʊ", "O")
		ps = strings.ReplaceAll(ps, "ɜːɹ", "ɜɹ")
		ps = strings.ReplaceAll(ps, "ɜː", "ɜɹ")
		ps = strings.ReplaceAll(ps, "ɪə", "iə")
		ps = strings.ReplaceAll(ps, "ː", "")
	}
	ps = strings.ReplaceAll(ps, "o", "ɔ")
	ps = strings.ReplaceAll(ps, "ɾ", "T")
	return strings.ReplaceAll(ps, "ʔ", "t")
}

func isBoundary(r rune) bool { return strings.ContainsRune(boundary, r) }

// syllabicConsonants turns "n̩" into "ᵊn" (the misaki spelling of a syllabic
// consonant) and drops any leftover combining vertical line.
func syllabicConsonants(ps string) string {
	rs := []rune(ps)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if rs[i] == '̩' {
			continue
		}
		if i+1 < len(rs) && rs[i+1] == '̩' && !unicode.IsSpace(rs[i]) {
			b.WriteRune('ᵊ')
			b.WriteRune(rs[i])
			i++
			continue
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}

// finalSyllabicL rewrites word-final "əl" as "ᵊl" (naval → nˈAvᵊl).
func finalSyllabicL(ps string) string {
	rs := []rune(ps)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if i+1 < len(rs) && rs[i] == 'ə' && rs[i+1] == 'l' {
			if i+2 >= len(rs) || isBoundary(rs[i+2]) {
				b.WriteString("ᵊl")
				i++
				continue
			}
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}

// replaceOpenA keeps misaki's ɐ for the article "a" (and British "an").
func replaceOpenA(ps string, british bool) string {
	rs := []rune(ps)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if rs[i] != 'ɐ' {
			b.WriteRune(rs[i])
			continue
		}
		j := i + 1
		if british && j < len(rs) && rs[j] == 'n' {
			j++
		}
		if j >= len(rs) || isBoundary(rs[j]) {
			b.WriteRune('ɐ')
		} else {
			b.WriteRune('ə')
		}
	}
	return b.String()
}

// theBeforeVowel turns "ðɪ" into "ði" for the word "the" before a vowel.
func theBeforeVowel(ps string) string {
	rs := []rune(ps)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if i+1 < len(rs) && rs[i] == 'ð' && rs[i+1] == 'ɪ' {
			beforeOK := i == 0 || unicode.IsSpace(rs[i-1])
			after := rune(0)
			if i+2 < len(rs) {
				after = rs[i+2]
			}
			afterOK := after == 0 || unicode.IsSpace(after)
			if beforeOK && afterOK {
				b.WriteString("ði")
				i++
				continue
			}
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}
