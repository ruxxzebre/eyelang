package interp

import (
	"reflect"
	goruntime "runtime"
	"sync"
)

// GoSource is where a function the interpreter provides is written in Go.
type GoSource struct {
	File string // as the compiler saw it: absolute, unless built with -trimpath
	Line int
}

var builtinSources = sync.OnceValue(builtinTable)

// BuiltinSource returns where the built-in function name (print,
// http.get, log.info) is implemented.
func BuiltinSource(name string) (GoSource, bool) {
	if b, ok := builtinSources()[name]; ok {
		return goSource(b.Fn)
	}
	return GoSource{}, false
}

// HostSource returns where the host function behind @host(name) is
// implemented.
func HostSource(name string) (GoSource, bool) {
	if h, ok := hostFuncs[name]; ok {
		return goSource(h)
	}
	return GoSource{}, false
}

func goSource(fn any) (GoSource, bool) {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func || v.IsNil() {
		return GoSource{}, false
	}
	f := goruntime.FuncForPC(v.Pointer())
	if f == nil {
		return GoSource{}, false
	}
	file, line := f.FileLine(f.Entry())
	return GoSource{File: file, Line: line}, file != ""
}
