package ir

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"strings"
)

// SiteFile is a file's name in a site, a call a compiled program names for
// the runner to evaluate: relative to the program's root, or as it is for a
// file of the standard library.
func SiteFile(prog *Program, f *File) string {
	if f == nil {
		return ""
	}
	// A module of the program's own may be above the root, as eye.toml's
	// `modules = ["../.."]` finds it. The standard library's have no directory.
	for _, m := range prog.Modules {
		if m.Dir == "" || prog.Root == nil {
			continue
		}
		for _, mf := range m.Files {
			if mf == f {
				// Without its leading ../: the root is deeper where the
				// program was built than in the copy its binary carries.
				if rel, err := filepath.Rel(prog.Root.Dir, f.Path); err == nil {
					rel = filepath.ToSlash(rel)
					for strings.HasPrefix(rel, "../") {
						rel = rel[len("../"):]
					}
					return rel
				}
			}
		}
	}
	return f.Path
}

// SiteKind is what an expression is as a site, or "" if the runner can't
// evaluate it for compiled code.
func SiteKind(e Expr) string {
	switch e.(type) {
	case *Call:
		return "call"
	case *MethodCall:
		return "method"
	case *Field:
		return "field"
	}
	return ""
}

// LambdaHash names a lambda by its typed IR and the slots its captures go
// in, for a saved function value: a lambda whose code changed under a paused
// run isn't found.
func LambdaHash(fn *Func) string {
	var b bytes.Buffer
	Fprint(&b, fn)
	for _, c := range fn.Captures {
		fmt.Fprintf(&b, "capture #%d\n", c.Slot)
	}
	sum := fnv.New64a()
	sum.Write(b.Bytes())
	return fmt.Sprintf("%016x", sum.Sum64())
}
