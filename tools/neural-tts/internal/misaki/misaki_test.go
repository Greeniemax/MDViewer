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
