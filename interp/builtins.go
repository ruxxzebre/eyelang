package interp

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"eyelang/ir"
)

type builtinFunc = func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value

// builtinTable returns the functions in ir.Builtins and the standard modules.
func builtinTable() map[string]*BuiltinFn {
	t := map[string]*BuiltinFn{}
	add := func(name string, fn builtinFunc) { t[name] = &BuiltinFn{Name: name, Fn: fn} }
	scheduleBuiltins(add)
	taskBuiltins(add)
	channelBuiltins(add)
	stringBuiltins(add)

	add("print", func(in *Interp, args []Value, _ map[string]Value, _ ir.Pos) Value {
		if in.replaying() {
			return ir.Unit{}
		}
		var parts []string
		for _, a := range args {
			parts = append(parts, in.display(a))
		}
		fmt.Fprintln(in.out, strings.Join(parts, " "))
		return ir.Unit{}
	})
	add("unreachable", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		why := "unreachable"
		if len(args) == 1 {
			why = "unreachable: " + show(args[0])
		}
		in.faultf(at, "%s", why)
		return nil
	})
	add("expect", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "expect", at)
		if !in.truth(args[0], at) {
			in.faultf(at, "expectation failed")
		}
		return ir.Unit{}
	})
	add("advance", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "advance", at)
		d, ok := args[0].(ir.Duration)
		if !ok || d < 0 {
			in.faultf(at, "advance(…) takes a Duration that isn't negative, such as 21 min")
		}
		if in.rt.real {
			in.faultf(at, "advance(…) moves a test's clock; it works only in tests")
		}
		in.rt.advance(d)
		return ir.Unit{}
	})
	add("fakes", func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
		h := &fakesHandler{}
		for _, a := range args {
			m, ok := a.(*ModuleVal)
			if !ok {
				in.faultf(at, "fakes(…) takes contract modules, such as fakes(notify, from: \"receipts/\")")
			}
			h.modules = append(h.modules, m.Module)
		}
		dir, ok := named["from"].(string)
		if !ok {
			in.faultf(at, "fakes(…) needs from: \"directory/\", where the recordings are")
		}
		h.dir = dir
		return h
	})
	add("clock", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "clock", at)
		when, ok := args[0].(time.Time)
		if !ok {
			in.faultf(at, "clock(…) takes an Instant, such as 2026-10-10 12:00 in \"Europe/Dublin\"")
		}
		return &clockHandler{when}
	})
	add("sleep", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "sleep", at)
		d, ok := args[0].(ir.Duration)
		if !ok {
			in.faultf(at, "sleep takes a Duration")
		}
		in.pause(time.Duration(d))
		return ir.Unit{}
	})
	add("sleepUntil", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "sleepUntil", at)
		t, ok := args[0].(time.Time)
		if !ok {
			in.faultf(at, "sleepUntil takes an Instant")
		}
		now := in.rt.now
		if in.rt.real {
			now = time.Now()
		}
		if t.After(now) {
			in.pause(t.Sub(now))
		}
		return ir.Unit{}
	})
	add("now", func(in *Interp, _ []Value, _ map[string]Value, _ ir.Pos) Value {
		if in.rt.real {
			return time.Now()
		}
		return in.rt.now
	})

	// Collections. Each takes the collection first, so it reads well after |>.
	add("size", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "size", at)
		return int64(len(in.elems(args[0], at)))
	})
	add("isEmpty", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "isEmpty", at)
		return len(in.elems(args[0], at)) == 0
	})
	add("add", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "add", at)
		switch c := args[0].(type) {
		case *Set:
			if c.has(args[1]) {
				return c
			}
			return newSet(append(append([]Value(nil), c.Elems...), args[1]))
		case Empty:
			return newSet([]Value{args[1]})
		case *List:
			return &List{Elems: append(append([]Value(nil), c.Elems...), args[1])}
		}
		in.faultf(at, "add takes a Set or a List, and this is %s", kindName(args[0]))
		return nil
	})
	add("remove", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "remove", at)
		k := keyOf(args[1])
		switch c := args[0].(type) {
		case *Set:
			var out []Value
			for _, e := range c.Elems {
				if keyOf(e) != k {
					out = append(out, e)
				}
			}
			return newSet(out)
		case *Map:
			var keys, vals []Value
			for i, e := range c.Keys {
				if keyOf(e) != k {
					keys, vals = append(keys, e), append(vals, c.Vals[i])
				}
			}
			return newMap(keys, vals)
		case Empty:
			return c
		}
		in.faultf(at, "remove takes a Set or a Map, and this is %s", kindName(args[0]))
		return nil
	})
	add("contains", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "contains", at)
		switch c := args[0].(type) {
		case *Set:
			return c.has(args[1])
		case *Map:
			_, ok := c.get(args[1])
			return ok
		case string:
			s, ok := args[1].(string)
			if !ok {
				in.faultf(at, "a string contains a String")
			}
			return strings.Contains(c, s)
		case *RangeVal:
			if t, ok := args[1].(ir.TimeOfDay); ok {
				if in, ok := inTimeRange(c, t); ok {
					return in
				}
			}
		}
		for _, e := range in.elems(args[0], at) {
			if equal(e, args[1]) {
				return true
			}
		}
		return false
	})
	add("insert", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 3, "insert", at)
		switch c := args[0].(type) {
		case *Map:
			return c.with(args[1], args[2])
		case Empty:
			return newMap([]Value{args[1]}, []Value{args[2]})
		}
		in.faultf(at, "insert takes a Map, and this is %s", kindName(args[0]))
		return nil
	})
	add("get", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "get", at)
		return in.index(args[0], args[1], at)
	})
	add("first", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "first", at)
		es := in.elems(args[0], at)
		if len(es) == 0 {
			return in.none()
		}
		return in.some(es[0])
	})
	add("last", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "last", at)
		es := in.elems(args[0], at)
		if len(es) == 0 {
			return in.none()
		}
		return in.some(es[len(es)-1])
	})
	add("map", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "map", at)
		if v, ok := args[0].(*Variant); ok {
			switch v.V.Name {
			case "Some", "Ok":
				return &Variant{V: v.V, Vals: []Value{in.callValue(args[1], at, v.Vals[0])}}
			}
			return v
		}
		var out []Value
		for _, e := range in.elems(args[0], at) {
			out = append(out, in.callValue(args[1], at, e))
		}
		return in.like(args[0], out)
	})
	add("filter", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "filter", at)
		var out []Value
		for _, e := range in.elems(args[0], at) {
			if in.truth(in.callValue(args[1], at, e), at) {
				out = append(out, e)
			}
		}
		return in.like(args[0], out)
	})
	add("find", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "find", at)
		for _, e := range in.elems(args[0], at) {
			if in.truth(in.callValue(args[1], at, e), at) {
				return in.some(e)
			}
		}
		return in.none()
	})
	add("any", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "any", at)
		for _, e := range in.elems(args[0], at) {
			if in.truth(in.callValue(args[1], at, e), at) {
				return true
			}
		}
		return false
	})
	add("all", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "all", at)
		for _, e := range in.elems(args[0], at) {
			if !in.truth(in.callValue(args[1], at, e), at) {
				return false
			}
		}
		return true
	})
	add("count", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		if len(args) == 1 {
			return int64(len(in.elems(args[0], at)))
		}
		in.need(args, 2, "count", at)
		var n int64
		for _, e := range in.elems(args[0], at) {
			if in.truth(in.callValue(args[1], at, e), at) {
				n++
			}
		}
		return n
	})
	add("reduce", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 3, "reduce", at)
		acc := args[1]
		for _, e := range in.elems(args[0], at) {
			acc = in.callValue(args[2], at, acc, e)
		}
		return acc
	})
	add("sum", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "sum", at)
		var acc Value = int64(0)
		for i, e := range in.elems(args[0], at) {
			if i == 0 {
				acc = e
				continue
			}
			acc = in.binary("+", acc, e, at)
		}
		return acc
	})
	add("enumerate", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "enumerate", at)
		var out []Value
		for i, e := range in.elems(args[0], at) {
			out = append(out, &Tuple{Elems: []Value{int64(i), e}})
		}
		return &List{Elems: out}
	})
	add("drop", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "drop", at)
		n := in.count(args[1], at)
		if s, ok := args[0].(string); ok { // characters, not bytes
			return s[charOffset(s, n):]
		}
		es := in.elems(args[0], at)
		if n > len(es) {
			n = len(es)
		}
		return in.like(args[0], es[n:])
	})
	add("take", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "take", at)
		n := in.count(args[1], at)
		if s, ok := args[0].(string); ok {
			return s[:charOffset(s, n)]
		}
		es := in.elems(args[0], at)
		if n > len(es) {
			n = len(es)
		}
		return in.like(args[0], es[:n])
	})
	add("reverse", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "reverse", at)
		es := in.elems(args[0], at)
		out := make([]Value, len(es))
		for i, e := range es {
			out[len(es)-1-i] = e
		}
		return &List{Elems: out}
	})
	add("sort", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "sort", at)
		out := append([]Value(nil), in.elems(args[0], at)...)
		sort.SliceStable(out, func(i, j int) bool {
			c, ok := compare(out[i], out[j])
			if !ok {
				in.faultf(at, "%s values can't be sorted", kindName(out[i]))
			}
			return c < 0
		})
		return &List{Elems: out}
	})
	add("keys", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "keys", at)
		if m, ok := args[0].(*Map); ok {
			return &List{Elems: append([]Value(nil), m.Keys...)}
		}
		return &List{}
	})
	add("values", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "values", at)
		if m, ok := args[0].(*Map); ok {
			return &List{Elems: append([]Value(nil), m.Vals...)}
		}
		return &List{}
	})
	add("entries", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "entries", at)
		return &List{Elems: in.elems(args[0], at)}
	})
	add("toList", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "toList", at)
		return &List{Elems: append([]Value(nil), in.elems(args[0], at)...)}
	})
	add("toSet", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "toSet", at)
		return newSet(in.elems(args[0], at))
	})
	add("join", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		sep := ""
		if len(args) == 2 {
			s, ok := args[1].(string)
			if !ok {
				in.faultf(at, "join's separator is a String")
			}
			sep = s
		}
		var parts []string
		for _, e := range in.elems(args[0], at) {
			parts = append(parts, show(e))
		}
		return strings.Join(parts, sep)
	})
	add("flatMap", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "flatMap", at)
		var out []Value
		for _, e := range in.elems(args[0], at) {
			out = append(out, in.elems(in.callValue(args[1], at, e), at)...)
		}
		return &List{Elems: out}
	})
	add("zip", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "zip", at)
		a, b := in.elems(args[0], at), in.elems(args[1], at)
		var out []Value
		for i := 0; i < len(a) && i < len(b); i++ {
			out = append(out, &Tuple{Elems: []Value{a[i], b[i]}})
		}
		return &List{Elems: out}
	})

	// Strings and numbers.
	add("lines", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		s := in.str(args, "lines", at)
		var out []Value
		for _, l := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
			out = append(out, strings.TrimSuffix(l, "\r"))
		}
		if s == "" {
			out = nil
		}
		return &List{Elems: out}
	})
	add("split", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "split", at)
		s, ok1 := args[0].(string)
		sep, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			in.faultf(at, "split takes two Strings")
		}
		var out []Value
		for _, p := range strings.Split(s, sep) {
			out = append(out, p)
		}
		return &List{Elems: out}
	})
	add("trim", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		return strings.TrimSpace(in.str(args, "trim", at))
	})
	add("toJson", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "toJson", at)
		out, err := json.Marshal(in.toWire(args[0], false))
		if err != nil {
			in.faultf(at, "toJson: %v", err)
		}
		return string(out)
	})
	// poll! in a job is handled by durable, which records its steps.
	add("poll", func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
		every, ok1 := named["every"].(ir.Duration)
		within, ok2 := named["within"].(ir.Duration)
		if len(args) != 1 || !ok1 || !ok2 || every <= 0 {
			in.faultf(at, "poll(every: …, within: …, fn { … }) takes two Durations and a function")
		}
		for waited := ir.Duration(0); ; waited += every {
			if r, _ := in.apply(args[0], nil, at); r == true {
				return true
			}
			if waited+every > within {
				return false
			}
			in.pause(time.Duration(every))
		}
	})
	// reserve! in a job is handled by durable, which records the grant.
	add("reserve", func(in *Interp, _ []Value, _ map[string]Value, at ir.Pos) Value {
		in.faultf(at, "reserve waits for memory from the runner, so only a job can call it, as reserve!(…)")
		return nil
	})
	// next! in a job is handled by durable, which records the wait.
	add("next", func(in *Interp, _ []Value, _ map[string]Value, at ir.Pos) Value {
		in.faultf(at, "next waits for an event, so only a job can call it, as next!(…)")
		return nil
	})
	// approval! in a job is handled by durable, which records the wait.
	add("approval", func(in *Interp, _ []Value, _ map[string]Value, at ir.Pos) Value {
		in.faultf(at, "approval waits for a person, so only a job can call it, as approval!(…)")
		return nil
	})
	answer := func(name string, approved bool) {
		add(name, func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
			what, ok := "", false
			if len(args) == 1 {
				what, ok = args[0].(string)
			}
			if !ok {
				in.faultf(at, "%s takes text from what was asked: %s(\"delete 3 backups\")", name, name)
			}
			by, ok := named["by"]
			if !ok {
				by = "test"
			}
			in.rt.answer(&Record{Names: []string{"what", "approved", "by"}, Vals: []Value{what, approved, by}})
			return ir.Unit{}
		})
	}
	answer("approve", true)
	answer("decline", false)
	add("startsWith", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "startsWith", at)
		s, ok1 := args[0].(string)
		p, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			in.faultf(at, "startsWith takes two Strings")
		}
		return strings.HasPrefix(s, p)
	})
	add("lower", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		return strings.ToLower(in.str(args, "lower", at))
	})
	add("upper", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		return strings.ToUpper(in.str(args, "upper", at))
	})
	isVariant := func(name string, want func(in *Interp) *ir.Variant) {
		add(name, func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
			in.need(args, 1, name, at)
			v, ok := args[0].(*Variant)
			return ok && v.V == want(in)
		})
	}
	add("mapErr", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "mapErr", at)
		r, ok := args[0].(*Variant)
		if !ok || (r.V != in.pre.ok && r.V != in.pre.err) {
			in.faultf(at, "mapErr works on a Result")
		}
		if r.V == in.pre.ok {
			return r
		}
		e, _ := in.apply(args[1], []argVal{{v: r.Vals[0]}}, at)
		return in.errV(e)
	})
	isVariant("isOk", func(in *Interp) *ir.Variant { return in.pre.ok })
	isVariant("isErr", func(in *Interp) *ir.Variant { return in.pre.err })
	add("nonzero", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "nonzero", at)
		if equal(args[0], int64(0)) {
			return in.none()
		}
		return in.some(args[0])
	})
	add("min", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "min", at)
		if c, ok := compare(args[0], args[1]); ok && c > 0 {
			return args[1]
		}
		return args[0]
	})
	add("max", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "max", at)
		if c, ok := compare(args[0], args[1]); ok && c < 0 {
			return args[1]
		}
		return args[0]
	})
	add("abs", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "abs", at)
		if c, ok := compare(args[0], int64(0)); ok && c < 0 {
			return in.unary("-", args[0], at)
		}
		if f, ok := args[0].(float64); ok && f < 0 {
			return -f
		}
		return args[0]
	})
	for _, name := range []string{"enters", "leaves"} {
		add(name, func(in *Interp, _ []Value, _ map[string]Value, at ir.Pos) Value {
			in.faultf(at, "%s(…) is a trigger, used inside @on(…)", name)
			return nil
		})
	}

	add("secret", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		name, ok := "", false
		if len(args) == 1 {
			name, ok = args[0].(string)
		}
		if !ok {
			in.faultf(at, `secret takes the secret's name: secret("notify-token")`)
		}
		return in.secret(name, at)
	})
	add("env.get", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		name, ok := "", false
		if len(args) == 1 {
			name, ok = args[0].(string)
		}
		if !ok {
			in.faultf(at, `env.get takes a variable's name: env.get("VPN_PROXY_URL")`)
		}
		if v, set := os.LookupEnv(name); set {
			return in.some(v)
		}
		return in.none()
	})

	for module, fns := range ir.BuiltinModules {
		for _, fn := range fns {
			if module == "log" {
				level := fn
				add("log."+fn, func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
					if in.replaying() {
						return ir.Unit{}
					}
					var parts []string
					for _, a := range args {
						parts = append(parts, in.display(a))
					}
					fmt.Fprintf(in.log, "%s: %s\n", level, strings.Join(parts, " "))
					return ir.Unit{}
				})
			}
		}
	}
	return t
}

// replaying reports whether a job run is going over steps it has already
// taken, so output it printed then isn't printed again.
func (in *Interp) replaying() bool {
	return in.run != nil && in.run.cursor < len(in.run.Log)
}

// clockHandler is the value of `clock(…)`: it sets a test's clock.
type clockHandler struct{ when time.Time }

func (h *clockHandler) apply(in *Interp, _ ir.Pos) { in.rt.now = h.when }

func (in *Interp) need(args []Value, n int, name string, at ir.Pos) {
	if len(args) != n {
		in.faultf(at, "%s takes %d arguments, and has %d", name, n, len(args))
	}
}

func (in *Interp) str(args []Value, name string, at ir.Pos) string {
	in.need(args, 1, name, at)
	s, ok := args[0].(string)
	if !ok {
		in.faultf(at, "%s takes a String, and this is %s", name, kindName(args[0]))
	}
	return s
}

func (in *Interp) count(v Value, at ir.Pos) int {
	n, ok := v.(int64)
	if !ok || n < 0 {
		in.faultf(at, "a count is an Int that isn't negative")
	}
	return int(n)
}

// elems returns the elements of anything a pipeline can walk.
func (in *Interp) elems(v Value, at ir.Pos) []Value {
	switch c := v.(type) {
	case *List:
		return c.Elems
	case *Set:
		return c.Elems
	case *Tuple:
		return c.Elems
	case *Map:
		out := make([]Value, len(c.Keys))
		for i, k := range c.Keys {
			out[i] = &Tuple{Elems: []Value{k, c.Vals[i]}}
		}
		return out
	case Empty:
		return nil
	case *EntitySet:
		return in.rt.all(c.Entity, at).Elems
	case *RangeVal:
		lo, ok1 := c.Start.(int64)
		hi, ok2 := c.End.(int64)
		if !ok1 || !ok2 {
			in.faultf(at, "a range of Ints is needed here")
		}
		if c.Inclusive {
			hi++
		}
		var out []Value
		for i := lo; i < hi; i++ {
			out = append(out, i)
		}
		return out
	case string:
		var out []Value
		for _, r := range c {
			out = append(out, r)
		}
		return out
	}
	in.faultf(at, "%s isn't a collection", kindName(v))
	return nil
}

// like builds a result of the same kind as the collection it came from: a
// set from a set, a list from anything else.
func (in *Interp) like(from Value, elems []Value) Value {
	if _, ok := from.(*Set); ok {
		return newSet(elems)
	}
	return &List{Elems: elems}
}

// callValue calls a function value from a builtin. A lambda with as many
// parameters as a tuple has elements accepts the tuple.
func (in *Interp) callValue(fn Value, at ir.Pos, args ...Value) Value {
	if c, ok := fn.(*Closure); ok && len(args) == 1 && len(c.Fn.Params) > 1 {
		if t, ok := args[0].(*Tuple); ok && len(t.Elems) == len(c.Fn.Params) {
			args = t.Elems
		}
	}
	avs := make([]argVal, len(args))
	for i, a := range args {
		avs[i] = argVal{v: a}
	}
	r, _ := in.apply(fn, avs, at)
	return r
}
