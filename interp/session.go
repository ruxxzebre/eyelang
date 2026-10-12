package interp

import (
	"fmt"
	"io"
	"strings"
	"time"

	"eyelang/ir"
)

// Session runs a REPL's inputs. The REPL checks its session again after
// every input, which makes a new program; the session carries over what the
// inputs made: the bindings' values, and the store with its entities, runs
// and events, which the new program picks up as `eye run` does after a
// restart. Its clock is a test's: it moves only when the REPL advances it.
type Session struct {
	out, log io.Writer
	in       *Interp
	store    Store
	now      time.Time
	values   map[string]Value
}

// NewSession starts a session whose clock reads now.
func NewSession(out, log io.Writer, now time.Time) *Session {
	return &Session{out: out, log: log, now: now, values: map[string]Value{}}
}

// Load makes prog the session's program. Values made under the previous
// program are carried over by name: a value that can be saved is saved and
// read back, so its variants and entities are the new program's; a function
// keeps running the code it was made from.
func (s *Session) Load(prog *ir.Program) error {
	in := New(prog, s.out, s.log)
	in.session = s
	in.rt.now = s.now
	// The store is the one the runtime picks for the program, the
	// MemoryStore written in Eyelang, holding what the previous one held.
	fresh := in.rt.index.Store
	if old, ok := s.store.(memory); ok {
		fresh.Commit(old.snapshot())
		in.rt.useStore(fresh)
	}
	s.store = fresh
	if old := s.in; old != nil {
		for name, v := range s.values {
			if data, err := tryEncode(old, v); err == nil {
				if nv, err := tryDecode(in, data); err == nil {
					s.values[name] = nv
				}
			}
		}
	}
	s.in = in
	return s.capture(func() { in.rt.restore() })
}

// Call runs fn, which takes no arguments, and the job runs it starts until
// they finish or wait. A fault is returned as an error; so is one in a job
// it started.
func (s *Session) Call(fn *ir.Func) (v Value, err error) {
	err = s.capture(func() {
		v, _ = s.in.callFunc(fn, nil, nil, nil, fn.Pos)
		s.in.rt.drain()
	})
	return v, err
}

// Advance moves the session's clock, running what wakes on the way.
func (s *Session) Advance(d time.Duration) error {
	return s.capture(func() { s.in.rt.advance(ir.Duration(d)) })
}

// Now is the session's clock.
func (s *Session) Now() time.Time { return s.now }

// Store is the session's store, for `:runs`.
func (s *Session) Store() Store { return s.store }

// Value returns a binding's value, and Set sets it.
func (s *Session) Value(name string) (Value, bool) {
	v, ok := s.values[name]
	return v, ok
}

// Forget drops a binding.
func (s *Session) Forget(name string) { delete(s.values, name) }

// Show writes a value as the REPL prints it: strings in quotes.
func Show(v Value) string {
	var b strings.Builder
	writeShow(&b, v, true)
	return b.String()
}

// TypeString writes a type as Eyelang source.
func TypeString(t *ir.TypeExpr) string { return typeString(t) }

// capture runs f, turning a fault into an error, and keeps the clock.
func (s *Session) capture(f func()) (err error) {
	rt := s.in.rt
	failures := len(rt.failures)
	depth := s.in.depth
	defer func() {
		s.now = rt.now
		s.in.depth = depth // a fault unwinds calls without counting them off
		if r := recover(); r != nil {
			switch x := r.(type) {
			case *Fault:
				err = x
			case *CommitError:
				err = x
			default:
				panic(r)
			}
		}
		if err == nil && len(rt.failures) > failures {
			err = rt.failures[failures]
		}
		rt.failures = rt.failures[:failures]
	}()
	f()
	return nil
}

func tryEncode(in *Interp, v Value) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return in.encode(v, ir.Pos{}), nil
}

func tryDecode(in *Interp, data []byte) (v Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return in.decode(data, ir.Pos{}), nil
}

// The REPL's session declares two host functions: one gives a binding's
// value to the code of a later input, and one keeps a value under a name.
func init() {
	hostFuncs["repl.value"] = func(in *Interp, params map[string]Value, at ir.Pos) Value {
		name, _ := params["name"].(string)
		if in.session == nil {
			in.faultf(at, "repl.value works only in the REPL")
		}
		v, ok := in.session.values[name]
		if !ok {
			in.faultf(at, "%s has no value yet", name)
		}
		return v
	}
	hostFuncs["repl.save"] = func(in *Interp, params map[string]Value, at ir.Pos) Value {
		name, _ := params["name"].(string)
		if in.session == nil {
			in.faultf(at, "repl.save works only in the REPL")
		}
		in.session.values[name] = params["value"]
		return params["value"]
	}
}
