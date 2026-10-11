package syntax

// The tree-sitter grammar, compiled into eye. tree-sitter-eyelang/src holds
// the parser that `tree-sitter generate` writes from grammar.js, and the
// external scanner for line breaks and strings.

// #cgo CFLAGS: -std=c11 -I${SRCDIR}/../tree-sitter-eyelang/src
// #include "parser.c"
// #include "scanner.c"
import "C"

import (
	"sync"
	"unsafe"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// language is the Eyelang grammar.
var language = sync.OnceValue(func() *ts.Language {
	return ts.NewLanguage(unsafe.Pointer(C.tree_sitter_eyelang()))
})

// parsers are reused: making a tree-sitter parser allocates.
var parsers = sync.Pool{New: func() any {
	p := ts.NewParser()
	if err := p.SetLanguage(language()); err != nil {
		panic(err)
	}
	return p
}}

// tsParse parses src with the tree-sitter grammar. The caller closes the
// tree.
func tsParse(src []byte) *ts.Tree {
	p := parsers.Get().(*ts.Parser)
	defer parsers.Put(p)
	return p.Parse(src, nil)
}
