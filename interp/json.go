package interp

import (
	"encoding/json"
	"math/big"
	"time"

	"eyelang/ir"
)

// jsonOf converts a value to the shape its type's schema describes (SPECS.md
// §19): a record is an object; a variant is an object tagged with "kind", or
// the value itself for a type that wraps one value; `Some(x)` is x and `None`
// is null; collections are arrays, and a map with string keys is an object.
// Times, durations and sizes are written as Eyelang shows them.
func (in *Interp) jsonOf(v Value) any {
	switch x := v.(type) {
	case int64, float64, string, bool:
		return x
	case *big.Int:
		return json.Number(x.String())
	case rune:
		return string(x)
	case ir.Unit:
		return nil
	case ir.Duration, ir.Size, ir.Date, ir.TimeOfDay, time.Time:
		return show(x)
	case *List:
		return in.jsonList(x.Elems)
	case *Tuple:
		return in.jsonList(x.Elems)
	case *Set:
		return in.jsonList(x.Elems)
	case Empty:
		return []any{}
	case *Map:
		obj := map[string]any{}
		for i, k := range x.Keys {
			s, ok := k.(string)
			if !ok {
				pairs := make([]any, len(x.Keys))
				for j := range x.Keys {
					pairs[j] = []any{in.jsonOf(x.Keys[j]), in.jsonOf(x.Vals[j])}
				}
				return pairs
			}
			obj[s] = in.jsonOf(x.Vals[i])
		}
		return obj
	case *Record:
		obj := map[string]any{}
		for i, name := range x.Names {
			obj[name] = in.jsonOf(x.Vals[i])
		}
		return obj
	case *Variant:
		switch {
		case x.V == in.pre.none:
			return nil
		case x.V == in.pre.some:
			return in.jsonOf(x.Vals[0])
		case len(x.V.Type.Variants) == 1 && len(x.Vals) == 1:
			return in.jsonOf(x.Vals[0])
		}
		obj := map[string]any{"kind": x.V.Name}
		for i, val := range x.Vals {
			name := "value"
			if i < len(x.V.Fields) && x.V.Fields[i].Name != "" {
				name = x.V.Fields[i].Name
			} else if len(x.Vals) > 1 {
				name = "value" + string(rune('1'+i))
			}
			obj[name] = in.jsonOf(val)
		}
		return obj
	}
	return show(v)
}

func (in *Interp) jsonList(elems []Value) []any {
	out := make([]any, len(elems))
	for i, e := range elems {
		out[i] = in.jsonOf(e)
	}
	return out
}
