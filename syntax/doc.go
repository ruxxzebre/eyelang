// Package syntax holds the Eyelang lexer, the lossless parser, the syntax tree
// and the formatter (phase 1 in ROADMAP.md).
//
// The syntax is defined by tree-sitter-eyelang/grammar.js. This package is a
// hand-written implementation of the same grammar, and the conformance suite in
// testdata/syntax checks that the two agree.
package syntax
