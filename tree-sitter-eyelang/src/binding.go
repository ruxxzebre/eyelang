// Package tree_sitter_eyelang is the Eyelang grammar for tree-sitter's Go
// bindings. eye's own parser is the hand-written one in syntax/; the syntax
// package's tests use this grammar as an oracle, so the eye binary doesn't link
// it and builds without cgo.
//
// The package lives in src/, beside parser.c and scanner.c, rather than in
// bindings/go as tree-sitter lays it out: Go compiles the C files of a
// package's own directory and rebuilds when they change, but not a file
// #included from elsewhere, so a regenerated grammar would go unnoticed.
package tree_sitter_eyelang

// #cgo CFLAGS: -std=c11 -fPIC
// const void *tree_sitter_eyelang(void);
import "C"

import "unsafe"

// Language returns the grammar, for tree_sitter.NewLanguage.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_eyelang())
}
