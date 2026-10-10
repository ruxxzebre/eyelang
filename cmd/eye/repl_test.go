package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestREPL runs eye repl on piped input: no prompts, one answer per input,
// an input that spans lines, and :quit.
func TestREPL(t *testing.T) {
	old := stdin
	stdin = strings.NewReader("let n = 20\nfn twice(x: Int) -> Int {\n  x * 2\n}\ntwice(n) + 2\n:quit\nnot reached\n")
	defer func() { stdin = old }()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"repl"}, &stdout, &stderr); code != 0 {
		t.Fatalf("eye repl: exit %d: %s", code, stderr.String())
	}
	want := "n: Int = 20\ndefined twice\n42: Int\n"
	if stdout.String() != want {
		t.Errorf("eye repl printed\n%s\nwant\n%s", stdout.String(), want)
	}
}
