package tts

import "testing"

func TestLengthScale(t *testing.T) {
	cases := []struct {
		rate int
		want float64
	}{
		{175, 1.0},     // natural pace
		{350, 0.5},     // clamped fast
		{50, 2.0},      // clamped slow (175/50=3.5 -> 2.0)
		{0, 1.0},       // guard against divide-by-zero
		{-10, 1.0},     // guard against negative
	}
	for _, c := range cases {
		if got := LengthScale(c.rate); got != c.want {
			t.Errorf("LengthScale(%d) = %v, want %v", c.rate, got, c.want)
		}
	}
	// 250 WPM -> 0.7 (within range), allow float tolerance.
	if got := LengthScale(250); got < 0.69 || got > 0.71 {
		t.Errorf("LengthScale(250) = %v, want ~0.7", got)
	}
}

func TestPiperVoiceID(t *testing.T) {
	if id, ok := PiperVoiceID("piper\ten_US-amy-medium"); !ok || id != "en_US-amy-medium" {
		t.Errorf("got (%q,%v), want (en_US-amy-medium,true)", id, ok)
	}
	if _, ok := PiperVoiceID("sapi\tMicrosoft David"); ok {
		t.Errorf("sapi selector should not be a piper voice")
	}
	if _, ok := PiperVoiceID("Alex"); ok {
		t.Errorf("bare macOS voice should not be a piper voice")
	}
}
