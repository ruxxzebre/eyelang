package cgen

import (
	"fmt"

	"eyelang/ir"
	"eyelang/lir"
)

// moreHosts calls the host functions of random (runtime/random.c) and
// fs.lines, as the interpreter's (interp/system.go), and reports whether
// f is one.
func (e *fn) moreHosts(x *lir.Runtime, f *ir.Func, args map[string]*lir.Value) bool {
	v := func(name string) string {
		a := args[name]
		if a == nil {
			e.errorf("%s takes %s", f.Host, name)
			return "0"
		}
		return e.v(a)
	}
	out := func() string { return "&" + e.v(x.Dst) }
	rt := func() string { return e.g.desc(x.Dst.Ty) }
	switch f.Host {
	case "random.int":
		e.printf("eyb_random_int(%s, %s, %s, %s);", e.at, v("range"), out(), rt())
	case "random.float":
		e.assign(x.Dst, "eyb_random_float()")
	case "random.pick":
		e.printf("eyb_random_pick(%s, %s, %s);", v("xs"), out(), rt())
	case "random.shuffle":
		e.assign(x.Dst, fmt.Sprintf("eyb_random_shuffle(%s)", v("xs")))
	case "fs.lines":
		e.printf("eyb_fs_lines(%s, %s, %s);", v("path"), out(), rt())
	default:
		return false
	}
	return true
}
