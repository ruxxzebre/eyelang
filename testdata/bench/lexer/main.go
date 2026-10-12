//go:build ignore

// The lexer benchmark, in Go: syntax/lexer.go on one big input, against the
// Eyelang lexer of testdata/programs/lexer on the same input in main.eye.
//
//	go run testdata/bench/lexer/main.go TIMES FILE...
//
// joins the files' text, repeats it TIMES times, lexes the result and prints
// the number of tokens and a checksum of their kinds, spans and flags: the
// line main.eye prints. The build tag keeps it out of `go build ./...`.
package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"eyelang/syntax"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: lexer TIMES FILE...")
	}
	times, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fail("usage: lexer TIMES FILE...: " + err.Error())
	}
	var text []byte
	for _, path := range os.Args[2:] {
		b, err := os.ReadFile(path)
		if err != nil {
			fail("reading " + path + ": " + err.Error())
		}
		text = append(text, b...)
	}
	input := bytes.Repeat(text, times)
	tokens, _ := syntax.Lex(input)
	fmt.Printf("%d tokens, checksum %d\n", len(tokens), checksum(tokens))
}

// checksum mixes each token's kind, its number in syntax/token.go, its span
// and its flags into a number below 1_000_000_007, as main.eye does.
func checksum(tokens []syntax.Token) int {
	sum := 0
	mix := func(n int) { sum = (sum*31 + n) % 1_000_000_007 }
	for _, t := range tokens {
		mix(int(t.Kind))
		mix(t.Span.Start)
		mix(t.Span.End)
		mix(int(t.Flags))
	}
	return sum
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
