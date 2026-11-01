package phonemes

import (
	"reflect"
	"testing"
)

func TestSplitClauses(t *testing.T) {
	got := Split("Hello, world. Is it faster? Yes!")
	want := []Clause{
		{Text: "Hello", Punct: ','},
		{Text: "world", Punct: '.'},
		{Text: "Is it faster", Punct: '?'},
		{Text: "Yes", Punct: '!'},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %#v, want %#v", got, want)
	}
}

func TestSplitSkipsEmptyClauses(t *testing.T) {
	got := Split("Wait... what")
	want := []Clause{
		{Text: "Wait", Punct: '.'},
		{Text: "what", Punct: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %#v, want %#v", got, want)
	}
}

func TestSplitEmpty(t *testing.T) {
	if got := Split(""); len(got) != 0 {
		t.Fatalf("Split(%q) = %#v, want empty", "", got)
	}
}

func TestSplitPunctuationOnly(t *testing.T) {
	if got := Split("...!?"); len(got) != 0 {
		t.Fatalf("Split(%q) = %#v, want empty", "...!?", got)
	}
}

func TestSplitLeadingPunctuation(t *testing.T) {
	got := Split(",Hello")
	// The leading delimiter has no preceding clause to attach to, so it is
	// dropped; the clause is final (Punct 0).
	want := []Clause{{Text: "Hello", Punct: 0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %#v, want %#v", got, want)
	}
}

func TestSplitTrailingPunctuation(t *testing.T) {
	got := Split("Hello,")
	want := []Clause{{Text: "Hello", Punct: ','}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %#v, want %#v", got, want)
	}
}

func TestSplitDropsQuotesOnly(t *testing.T) {
	got := Split("He said \"hi.\"")
	want := []Clause{{Text: "He said \"hi", Punct: '.'}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %#v, want %#v", got, want)
	}
}

func TestSplitDropsTrailingWrappedPunctuation(t *testing.T) {
	got := Split("(see \"x.\")")
	want := []Clause{{Text: "(see \"x", Punct: '.'}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %#v, want %#v", got, want)
	}
}

func TestIDsPadsAndDropsUnknown(t *testing.T) {
	vocab := map[string]int64{"a": 43, "b": 44, " ": 16}
	got := IDs("a?b", vocab, 510)
	want := []int64{0, 43, 44, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestIDsCapsAtMax(t *testing.T) {
	vocab := map[string]int64{"a": 43}
	got := IDs("aaaaaa", vocab, 3)
	want := []int64{0, 43, 43, 43, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestIDsEmptyInput(t *testing.T) {
	vocab := map[string]int64{"a": 43}
	got := IDs("", vocab, 510)
	want := []int64{0, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestIDsZeroMaxOnlyPads(t *testing.T) {
	vocab := map[string]int64{"a": 43}
	got := IDs("aaa", vocab, 0)
	want := []int64{0, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestIDsNegativeMaxOnlyPads(t *testing.T) {
	vocab := map[string]int64{"a": 43}
	got := IDs("aaa", vocab, -1)
	want := []int64{0, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestIDsMaxOne(t *testing.T) {
	vocab := map[string]int64{"a": 43}
	got := IDs("aaa", vocab, 1)
	want := []int64{0, 43, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestIDsAllUnknown(t *testing.T) {
	vocab := map[string]int64{"a": 43}
	got := IDs("??", vocab, 510)
	want := []int64{0, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestIDsMultibyteRune(t *testing.T) {
	vocab := map[string]int64{"ʤ": 82}
	got := IDs("ʤ", vocab, 510)
	want := []int64{0, 82, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestSplitPhonemesCutsAtSpace(t *testing.T) {
	got := SplitPhonemes("aa bb cc", 5)
	want := []string{"aa", "bb cc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitPhonemes = %q, want %q", got, want)
	}
}

func TestSplitPhonemesHardCut(t *testing.T) {
	got := SplitPhonemes("abcdef", 4)
	want := []string{"abcd", "ef"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitPhonemes = %q, want %q", got, want)
	}
}

func TestSplitPhonemesZeroMax(t *testing.T) {
	if got := SplitPhonemes("abc", 0); got != nil {
		t.Fatalf("SplitPhonemes(max=0) = %q, want nil", got)
	}
}

func TestSplitPhonemesNegativeMax(t *testing.T) {
	if got := SplitPhonemes("abc", -1); got != nil {
		t.Fatalf("SplitPhonemes(max=-1) = %q, want nil", got)
	}
}

func TestSplitPhonemesExactLength(t *testing.T) {
	got := SplitPhonemes("abcd", 4)
	want := []string{"abcd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitPhonemes = %q, want %q", got, want)
	}
}

func TestSplitPhonemesMaxOne(t *testing.T) {
	got := SplitPhonemes("ab", 1)
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitPhonemes = %q, want %q", got, want)
	}
}

func TestSplitPhonemesRuneBased(t *testing.T) {
	got := SplitPhonemes("ɜɹ ʤʤ", 2)
	want := []string{"ɜɹ", "ʤʤ"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitPhonemes = %q, want %q", got, want)
	}
}
