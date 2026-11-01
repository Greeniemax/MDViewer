package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestServeLoopHandlesRequests(t *testing.T) {
	var got []serveRequest
	in := strings.NewReader(`{"text":"hello","voice":"af_heart","speed":1.2,"out":"a.wav"}
{"text":"world","voice":"af_heart","out":"b.wav"}

`)
	var out bytes.Buffer

	err := serveLoop(in, &out, func(r serveRequest) error {
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "hello" || got[1].Out != "b.wav" {
		t.Fatalf("synth calls = %#v", got)
	}

	responses := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(responses) != 2 {
		t.Fatalf("responses = %q", out.String())
	}
	for _, line := range responses {
		var resp serveResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("bad response %q: %v", line, err)
		}
		if !resp.OK || resp.Error != "" {
			t.Fatalf("response = %#v", resp)
		}
	}
}

func TestServeLoopReportsSynthError(t *testing.T) {
	in := strings.NewReader(`{"text":"boom","out":"x.wav"}` + "\n")
	var out bytes.Buffer

	if err := serveLoop(in, &out, func(serveRequest) error {
		return errors.New("model exploded")
	}); err != nil {
		t.Fatal(err)
	}
	var resp serveResponse
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK || resp.Error != "model exploded" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestServeLoopContinuesAfterBadRequest(t *testing.T) {
	in := strings.NewReader("not json\n{\"text\":\"ok\",\"out\":\"y.wav\"}\n")
	var out bytes.Buffer

	calls := 0
	if err := serveLoop(in, &out, func(serveRequest) error {
		calls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("synth calls = %d, want 1", calls)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("responses = %q", out.String())
	}
	var bad serveResponse
	if err := json.Unmarshal([]byte(lines[0]), &bad); err != nil {
		t.Fatal(err)
	}
	if bad.OK || !strings.Contains(bad.Error, "bad request") {
		t.Fatalf("bad response = %#v", bad)
	}
	var good serveResponse
	if err := json.Unmarshal([]byte(lines[1]), &good); err != nil {
		t.Fatal(err)
	}
	if !good.OK {
		t.Fatalf("good response = %#v", good)
	}
}
