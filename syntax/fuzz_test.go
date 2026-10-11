package syntax

import (
	"strings"
	"testing"
)

// TestFuel checks that a parser that keeps looking without consuming a token
// stops with a message instead of hanging. FuzzParse, in parser_test.go,
// searches for inputs that make it run out.
func TestFuel(t *testing.T) {
	p := &parser{toks: []Token{{Kind: Ident}, {Kind: EOF}}, fuel: fuel}
	defer func() {
		r := recover()
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "stuck at offset 0") {
			t.Fatalf("recovered %v, want a stuck parser's message", r)
		}
	}()
	for {
		p.at(Ident)
	}
}
