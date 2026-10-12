package interp

import (
	"unicode/utf8"

	"eyelang/ir"
)

// IterVal is an Iter(a): a source collection and the lazy steps applied to
// it, in order (SPECS.md §5). It is a value: nothing in it changes, and each
// walk starts again from the source and runs the steps' functions again.
type IterVal struct {
	Src   Value
	Steps []iterStep
}

// iterStep is one lazy step of an Iter's pipeline.
type iterStep struct {
	op    iterOp
	fn    Value  // map, filter, flatMap
	n     int    // take, drop
	other Value  // zip: what the elements are paired with
	at    ir.Pos // where the step is written, for faults in its function
}

type iterOp int

const (
	iterMap iterOp = iota
	iterFilter
	iterTake
	iterDrop
	iterEnumerate
	iterZip
	iterFlatMap
)

// then is the Iter with one more step. The steps are copied, so the Iter it
// came from is unchanged.
func (it *IterVal) then(s iterStep) *IterVal {
	steps := make([]iterStep, len(it.Steps), len(it.Steps)+1)
	copy(steps, it.Steps)
	return &IterVal{Src: it.Src, Steps: append(steps, s)}
}

// iterOf is the Iter of a collection; an Iter is its own.
func iterOf(v Value) *IterVal {
	if it, ok := v.(*IterVal); ok {
		return it
	}
	return &IterVal{Src: v}
}

// cursor gives the elements of a walk one at a time: the next one, or false
// when there are no more.
type cursor func() (Value, bool)

// cursor starts a walk of anything a pipeline can walk. A range and a string
// give their elements as they are asked for; an Iter runs its steps as each
// element passes through them.
func (in *Interp) cursor(v Value, at ir.Pos) cursor {
	switch c := v.(type) {
	case *IterVal:
		cur := in.cursor(c.Src, at)
		for _, s := range c.Steps {
			cur = in.step(cur, s)
		}
		return cur
	case *RangeVal:
		lo, ok1 := c.Start.(int64)
		hi, ok2 := c.End.(int64)
		if !ok1 || !ok2 {
			in.faultf(at, "a range of Ints is needed here")
		}
		if c.Inclusive {
			hi++
		}
		i := lo
		return func() (Value, bool) {
			if i >= hi {
				return nil, false
			}
			i++
			return i - 1, true
		}
	case string:
		i := 0
		return func() (Value, bool) {
			if i >= len(c) {
				return nil, false
			}
			r, n := utf8.DecodeRuneInString(c[i:])
			i += n
			return r, true
		}
	case *Map:
		i := 0
		return func() (Value, bool) {
			if i >= len(c.Keys) {
				return nil, false
			}
			i++
			return &Tuple{Elems: []Value{c.Keys[i-1], c.Vals[i-1]}}, true
		}
	}
	es := in.elems(v, at)
	i := 0
	return func() (Value, bool) {
		if i >= len(es) {
			return nil, false
		}
		i++
		return es[i-1], true
	}
}

// step puts one step of a pipeline after a cursor.
func (in *Interp) step(from cursor, s iterStep) cursor {
	switch s.op {
	case iterMap:
		return func() (Value, bool) {
			v, ok := from()
			if !ok {
				return nil, false
			}
			return in.callValue(s.fn, s.at, v), true
		}
	case iterFilter:
		return func() (Value, bool) {
			for {
				v, ok := from()
				if !ok {
					return nil, false
				}
				if in.truth(in.callValue(s.fn, s.at, v), s.at) {
					return v, true
				}
			}
		}
	case iterTake:
		left := s.n
		return func() (Value, bool) {
			if left <= 0 {
				return nil, false // without asking for one more
			}
			left--
			return from()
		}
	case iterDrop:
		skip := s.n
		return func() (Value, bool) {
			for ; skip > 0; skip-- {
				if _, ok := from(); !ok {
					return nil, false
				}
			}
			return from()
		}
	case iterEnumerate:
		var i int64
		return func() (Value, bool) {
			v, ok := from()
			if !ok {
				return nil, false
			}
			i++
			return &Tuple{Elems: []Value{i - 1, v}}, true
		}
	case iterZip:
		other := in.cursor(s.other, s.at)
		return func() (Value, bool) {
			a, ok := from()
			if !ok {
				return nil, false
			}
			b, ok := other()
			if !ok {
				return nil, false
			}
			return &Tuple{Elems: []Value{a, b}}, true
		}
	case iterFlatMap:
		var inner cursor
		return func() (Value, bool) {
			for {
				if inner != nil {
					if v, ok := inner(); ok {
						return v, true
					}
				}
				v, ok := from()
				if !ok {
					return nil, false
				}
				inner = in.cursor(in.callValue(s.fn, s.at, v), s.at)
			}
		}
	}
	panic("unknown step of an Iter")
}

// walk calls each with the elements of v in order until it returns false.
func (in *Interp) walk(v Value, at ir.Pos, each func(Value) bool) {
	next := in.cursor(v, at)
	for {
		e, ok := next()
		if !ok || !each(e) {
			return
		}
	}
}

// collect walks an Iter to its end and gives its elements.
func (in *Interp) collect(it *IterVal, at ir.Pos) []Value {
	var out []Value
	in.walk(it, at, func(v Value) bool {
		out = append(out, v)
		return true
	})
	return out
}
