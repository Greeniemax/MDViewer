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
