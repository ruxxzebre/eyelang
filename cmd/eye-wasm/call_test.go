package main

import (
	"encoding/json"
	"strings"
	"testing"

	"eyelang/internal/playground"
)

// TestCall makes the calls the playground page makes, as JSON.
func TestCall(t *testing.T) {
	p := playground.New()
	if got := call(p, "setFiles", `{"files": {"main.eye": "pub fn main() uses {stdio} { print(\"hi\") }\n"}}`, nil); got != `{"result":true}` {
		t.Errorf("setFiles = %s", got)
	}
	if got := call(p, "check", "", nil); got != `{"result":[]}` {
		t.Errorf("check = %s", got)
	}
	var printed strings.Builder
	got := call(p, "run", `{"args": [], "start": "2026-10-12T09:00:00Z"}`, func(stream, text string) {
		printed.WriteString(stream + ": " + text)
	})
	if got != `{"result":{"diagnostics":[],"exit":0}}` || printed.String() != "stdout: hi\n" {
		t.Errorf("run = %s, printed %q", got, printed.String())
	}
	var hl struct {
		Result []playground.Span `json:"result"`
	}
	if err := json.Unmarshal([]byte(call(p, "highlight", `{"text": "fn f() = 1"}`, nil)), &hl); err != nil || len(hl.Result) != 2 || hl.Result[0].Class != "k" {
		t.Errorf("highlight = %+v, %v", hl, err)
	}
	if got := call(p, "format", `{"path": "nothing.eye"}`, nil); !strings.Contains(got, `"error"`) {
		t.Errorf("format of a missing file = %s", got)
	}
	if got := call(p, "fly", "", nil); got != `{"error":"no op \"fly\""}` {
		t.Errorf("fly = %s", got)
	}
}
