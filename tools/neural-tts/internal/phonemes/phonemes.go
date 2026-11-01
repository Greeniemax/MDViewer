// Package phonemes turns a line of text into model-ready token IDs.
package phonemes

import (
	"strings"
	"unicode"
)

// Clause is one punctuation-terminated segment of a line. Punct is the
// delimiter that followed it, or 0 for the final clause.
type Clause struct {
	Text  string
	Punct rune
}

var clauseDelimiters = map[rune]bool{
	',': true, '.': true, ';': true, ':': true,
	'!': true, '?': true, '—': true, '…': true,
}

// Split divides text at clause punctuation, keeping the delimiter so callers
// can re-attach it to the phoneme string (espeak drops punctuation).
func Split(text string) []Clause {
	var out []Clause
	var cur []rune

	flush := func(p rune) {
		t := strings.TrimSpace(string(cur))
		if speakable(t) {
			out = append(out, Clause{Text: t, Punct: p})
		}
		cur = cur[:0]
	}

	for _, r := range text {
		if clauseDelimiters[r] {
			flush(r)
			continue
		}
		cur = append(cur, r)
	}
	flush(0)
	return out
}

// speakable reports whether s contains something espeak can pronounce, so
// punctuation or quote leftovers after a clause delimiter are not emitted as
// their own clause.
func speakable(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// IDs maps phoneme runes to vocabulary IDs, drops unknown runes, caps the
// payload at max, and wraps it in the 0 pad token the models expect.
func IDs(ps string, vocab map[string]int64, max int) []int64 {
	if max <= 0 {
		return []int64{0, 0}
	}
	ids := make([]int64, 0, len(ps)+2)
	ids = append(ids, 0)
	n := 0
	for _, r := range ps {
		id, ok := vocab[string(r)]
		if !ok {
			continue
		}
		ids = append(ids, id)
		n++
		if n >= max {
			break
		}
	}
	return append(ids, 0)
}

// SplitPhonemes cuts a phoneme string into chunks of at most max runes,
// preferring the last space before the limit. This mirrors Paradee's _chunks.
func SplitPhonemes(ps string, max int) []string {
	if max <= 0 {
		return nil
	}
	rs := []rune(ps)
	var out []string
	for len(rs) > max {
		cut := max
		for i := max - 1; i > 0; i-- {
			if rs[i] == ' ' {
				cut = i
				break
			}
		}
		out = append(out, string(rs[:cut]))
		rs = []rune(strings.TrimLeft(string(rs[cut:]), " "))
	}
	if len(rs) > 0 {
		out = append(out, string(rs))
	}
	return out
}
