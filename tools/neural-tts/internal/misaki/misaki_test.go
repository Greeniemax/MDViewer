package misaki

import "testing"

func TestAmericanDiphthongsAndRhotics(t *testing.T) {
	got := ToMisaki("həlˈoʊ wˈɜːld", false)
	want := "həlˈO wˈɜɹld"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestBritishEaBeforeE(t *testing.T) {
	got := ToMisaki("ðeə", true)
	want := "ðɛː"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestSyllabicConsonant(t *testing.T) {
	got := ToMisaki("bˈʌt n̩", false)
	want := "bˈʌt ᵊn"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestWordFinalSyllabicL(t *testing.T) {
	got := ToMisaki("nˈeɪvəl", false)
	want := "nˈAvᵊl"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestAmericanFlapAndGlottal(t *testing.T) {
	got := ToMisaki("wˈɔɾɚ ʔ", false)
	want := "wˈɔTəɹ t"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestBritishTheBeforeVowel(t *testing.T) {
	got := ToMisaki("ðɪ æpl", true)
	want := "ði æpl"
	if got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestArticleAKeepsOpenA(t *testing.T) {
	if got, want := ToMisaki("ɐ æpl", false), "ɐ æpl"; got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
	if got, want := ToMisaki("ɐbˈʌv", false), "əbˈʌv"; got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}

func TestUnicodeBoundaries(t *testing.T) {
	const nbsp = string(rune(0x00A0)) // JS whitespace, absent from the ASCII boundary set
	cases := []struct{ name, in, want string }{
		{"space after syllabic l", "əl ", "ᵊl "},
		{"space after open a", "ɐ ", "ɐ "},
		{"nbsp after syllabic l", "əl" + nbsp, "ᵊl" + nbsp},
		{"nbsp after open a", "ɐ" + nbsp, "ɐ" + nbsp},
		{"bom after open a", "ɐ" + string(rune(0xFEFF)), "ɐ" + string(rune(0xFEFF))},
	}
	for _, tc := range cases {
		if got := ToMisaki(tc.in, false); got != tc.want {
			t.Errorf("%s: ToMisaki(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestReplacementTable(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"diphthongs", "aɪ aʊ", "I W"},
		{"affricates", "dʒ tʃ", "ʤ ʧ"},
		// ʲo maps to "jo" in the table, then the final o->ɔ pass lowers that
		// o too, so the observable result is "jɔ jə " (trailing ʲ is deleted).
		{"palatalized", "ʲo ʲə ʲ", "jɔ jə "},
		{"fricatives and lateral", "x ç ɬ", "k k l"},
		{"nasalization", "ɔɪ̃", "Y"},
		{"glottalized syllabic n", "ʔn̩", "tn"},
	}
	for _, tc := range cases {
		if got := ToMisaki(tc.in, false); got != tc.want {
			t.Errorf("%s: ToMisaki(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestReplacementOrder(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		british bool
		want    string
	}{
		{"rhotic not doubled", "ɜːɹ", false, "ɜɹ"},
		{"goat before o lowering", "əʊ", true, "Q"},
		{"syllabic before open a", "ɐ̩", false, "ᵊɐ"},
		{"british article an", "ɐn", true, "ɐn"},
		{"american open a", "ɐn", false, "ən"},
	}
	for _, tc := range cases {
		if got := ToMisaki(tc.in, tc.british); got != tc.want {
			t.Errorf("%s: ToMisaki(%q, %v) = %q, want %q", tc.name, tc.in, tc.british, got, tc.want)
		}
	}
}

func TestConsecutiveSyllabicMarks(t *testing.T) {
	if got, want := ToMisaki("̩̩", false), "ᵊ"; got != want {
		t.Fatalf("ToMisaki = %q, want %q", got, want)
	}
}
