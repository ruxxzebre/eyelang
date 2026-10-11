// Package tree_sitter_eyelang is the Eyelang grammar for tree-sitter's Go
// bindings. eye's own parser is the hand-written one in syntax/; the syntax
// package's tests use this grammar as an oracle, so the eye binary doesn't link
// it and builds without cgo.
package tree_sitter_eyelang

// #cgo CFLAGS: -std=c11 -fPIC
// #include "../../src/parser.c"
// #include "../../src/scanner.c"
import "C"

import "unsafe"

// Language returns the grammar, for tree_sitter.NewLanguage.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_eyelang())
}
