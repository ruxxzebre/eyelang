package interp

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"eyelang/ir"
)

// Values travel as JSON in one shape, the one a type's schema describes
// (SPECS.md §19): toJson writes it, contracts over HTTP send and read it, and
// model calls ask for it.
//
//   - a record is an object, and a type that wraps one value is that value;
//   - a type whose variants have no fields is a string, the variant's name;
//   - any other variant is an object tagged with "kind", with its fields;
//   - `Some(x)` is x, and `None` is null;
//   - lists, sets and tuples are arrays, and a map with string keys is an
//     object;
//   - an Instant is an RFC 3339 string, a Date "2026-10-12", a time of day
//     "07:40", a Duration a number of milliseconds and a Size of bytes.
//
// Providers' strict schemas name every property of an object and have no
// fixed-length arrays, so a model is asked for a map as a list of
// {"key": …, "value": …} entries and for a tuple as an object with fields
// value1, value2, …. Reading takes either shape.
//
// With snake set, names are snake_case on the wire: field dedupeKey is
// dedupe_key, and variant Normal is "normal".

// wireName is a field's or variant's name on the wire.
func wireName(name string, snake bool) string {
	if !snake {
		return name
	}
	var b strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isEnum reports whether a type's variants all have no fields, so that its
// values are strings on the wire.
func isEnum(t *ir.Type) bool {
	if len(t.Variants) == 0 {
		return false
	}
	for _, v := range t.Variants {
		if len(v.Fields) > 0 {
			return false
		}
	}
	return true
}

func (in *Interp) isOption(t *ir.Type) bool {
	return t != nil && in.pre.some != nil && t == in.pre.some.Type
}

// toWire converts a value to its JSON shape.
func (in *Interp) toWire(v Value, snake bool) any {
	switch x := v.(type) {
	case int64, float64, string, bool:
		return x
	case *big.Int:
		return json.Number(x.String())
	case rune:
		return string(x)
	case ir.Unit:
		return nil
	case ir.Duration:
		return int64(time.Duration(x) / time.Millisecond)
	case time.Time:
		return x.Format(time.RFC3339)
	case ir.Size:
		return int64(x)
	case ir.Date, ir.TimeOfDay:
		return show(x)
	case *List:
		return in.wireList(x.Elems, snake)
	case *Tuple:
		return in.wireList(x.Elems, snake)
	case *Set:
		return in.wireList(x.Elems, snake)
	case Empty:
		return []any{}
	case *Map:
		obj := map[string]any{}
		for i, k := range x.Keys {
			key, ok := in.toWire(k, snake).(string)
			if !ok {
				pairs := make([]any, len(x.Keys))
				for j := range x.Keys {
					pairs[j] = []any{in.toWire(x.Keys[j], snake), in.toWire(x.Vals[j], snake)}
				}
				return pairs
			}
			obj[key] = in.toWire(x.Vals[i], snake)
		}
		return obj
	case *Record:
		obj := map[string]any{}
		for i, name := range x.Names {
			obj[wireName(name, snake)] = in.toWire(x.Vals[i], snake)
		}
		return obj
	case *Variant:
		t := x.V.Type
		switch {
		case x.V == in.pre.none:
			return nil
		case x.V == in.pre.some:
			return in.toWire(x.Vals[0], snake)
		case t != nil && t.Newtype():
			return in.toWire(x.Vals[0], snake)
		case x.V == in.pre.stale:
			// It joins a command's own variants, which may have fields, so
			// it is tagged, as Expired and Uncertain are.
			return map[string]any{"kind": x.V.Name}
		case t != nil && isEnum(t):
			return wireName(x.V.Name, snake)
		}
		obj := map[string]any{"kind": x.V.Name}
		for i, val := range x.Vals {
			obj[wireName(fieldName(x.V, i, len(x.Vals)), snake)] = in.toWire(val, snake)
		}
		return obj
	case *SecretVal:
		return x.value
	case *ApprovalVal:
		return map[string]any{"what": x.What, "by": x.By}
	}
	return show(v)
}

// opResult reads an operation's result in its JSON shape, as type t. A
// @fresh command may also answer Stale or Expired: {"kind": "Stale"}, or
// the bare name.
func (in *Interp) opResult(op *ir.Op, x any, t *ir.TypeExpr, snake bool) (Value, error) {
	if v := in.freshOutcome(op, x, snake); v != nil {
		return v, nil
	}
	return in.fromWire(x, t, snake)
}

// freshOutcome is Stale or Expired when x says one of them and op is a
// @fresh command, and nil otherwise.
func (in *Interp) freshOutcome(op *ir.Op, x any, snake bool) Value {
	if !op.Fresh {
		return nil
	}
	for _, v := range []*ir.Variant{in.pre.stale, in.pre.expired} {
		switch y := x.(type) {
		case string:
			if y == wireName(v.Name, snake) {
				return &Variant{V: v}
			}
		case map[string]any:
			if y["kind"] == v.Name {
				return &Variant{V: v}
			}
		}
	}
	return nil
}

// fieldName is a variant field's name in JSON: its own, or "value" for a
// positional field ("value1", "value2", … when there are several).
func fieldName(v *ir.Variant, i, n int) string {
	if i < len(v.Fields) && v.Fields[i].Name != "" {
		return v.Fields[i].Name
	}
	if n == 1 {
		return "value"
	}
	return fmt.Sprintf("value%d", i+1)
}

func (in *Interp) wireList(elems []Value, snake bool) []any {
	out := make([]any, len(elems))
	for i, e := range elems {
		out[i] = in.toWire(e, snake)
	}
	return out
}

// typeEnv maps a generic type's variables to the types they stand for.
type typeEnv map[string]*ir.TypeExpr

func (env typeEnv) resolve(t *ir.TypeExpr) *ir.TypeExpr {
	if t != nil && t.Decl == nil && len(t.Args) == 0 {
		if r, ok := env[t.Name]; ok {
			return r
		}
	}
	return t
}

// bind returns the environment for a declaration used with arguments.
func bindParams(d *ir.Type, args []*ir.TypeExpr, outer typeEnv) typeEnv {
	env := typeEnv{}
	for i, p := range d.Params {
		if i < len(args) {
			env[p] = outer.resolve(args[i])
		}
	}
	return env
}

// fromWire reads a JSON value as type t. Fields a record doesn't declare are
// ignored, and a missing field is None when its type is an Option.
func (in *Interp) fromWire(x any, t *ir.TypeExpr, snake bool) (Value, error) {
	return in.decodeWire(x, t, snake, typeEnv{}, "")
}

func (in *Interp) decodeWire(x any, t *ir.TypeExpr, snake bool, env typeEnv, path string) (Value, error) {
	t = env.resolve(t)
	fail := func(want string) (Value, error) {
		at := ""
		if path != "" {
			at = path + ": "
		}
		return nil, fmt.Errorf("%sexpected %s, found %s", at, want, describeJSON(x))
	}
	if t == nil {
		return nil, fmt.Errorf("%s: no type to read the value as", path)
	}
	if t.Decl != nil {
		return in.decodeDecl(x, t.Decl, bindParams(t.Decl, t.Args, env), snake, path)
	}
	arg := func(i int) *ir.TypeExpr {
		if i < len(t.Args) {
			return env.resolve(t.Args[i])
		}
		return nil
	}
	switch t.Name {
	case "String", "Key", "Zone":
		if s, ok := x.(string); ok {
			return s, nil
		}
		return fail("a string")
	case "Int":
		if n, ok := x.(json.Number); ok {
			if b, ok := new(big.Int).SetString(n.String(), 10); ok {
				return normInt(b), nil
			}
		}
		return fail("a whole number")
	case "Version":
		// As the service gives it: a number, as a snapshot's, or a string.
		switch v := x.(type) {
		case json.Number:
			if b, ok := new(big.Int).SetString(v.String(), 10); ok {
				return normInt(b), nil
			}
		case string:
			return v, nil
		}
		return fail("a version, a whole number or a string")
	case "Float":
		if n, ok := x.(json.Number); ok {
			if f, err := n.Float64(); err == nil {
				return f, nil
			}
		}
		return fail("a number")
	case "Bool":
		if b, ok := x.(bool); ok {
			return b, nil
		}
		return fail("true or false")
	case "Duration":
		if n, ok := x.(json.Number); ok {
			if ms, err := n.Int64(); err == nil {
				return ir.Duration(time.Duration(ms) * time.Millisecond), nil
			}
		}
		return fail("a number of milliseconds")
	case "Size":
		if n, ok := x.(json.Number); ok {
			if b, err := n.Int64(); err == nil {
				return ir.Size(b), nil
			}
		}
		return fail("a number of bytes")
	case "Instant":
		switch v := x.(type) {
		case string:
			if when, err := time.Parse(time.RFC3339, v); err == nil {
				return when.UTC(), nil
			}
		case json.Number:
			if secs, err := v.Int64(); err == nil {
				return time.Unix(secs, 0).UTC(), nil
			}
		}
		return fail("an RFC 3339 time")
	case "Date":
		if s, ok := x.(string); ok {
			if d, err := time.Parse("2006-01-02", s); err == nil {
				return ir.Date{Year: d.Year(), Month: int(d.Month()), Day: d.Day()}, nil
			}
		}
		return fail("a date such as 2026-10-12")
	case "Time":
		if s, ok := x.(string); ok && timeOfDayRE.MatchString(s) {
			for _, layout := range []string{"15:04", "15:04:05"} {
				if d, err := time.Parse(layout, s); err == nil {
					return ir.TimeOfDay{Hour: d.Hour(), Minute: d.Minute(), Second: d.Second()}, nil
				}
			}
		}
		return fail("a time of day such as 14:30")
	case "tuple":
		// An array, as toWire writes it, or an object with fields value1,
		// value2, …, as schemaOf asks a model for it.
		vals := make([]Value, len(t.Args))
		xs, isArray := x.([]any)
		obj, isObject := x.(map[string]any)
		if (!isArray || len(xs) != len(vals)) && !isObject {
			return fail(fmt.Sprintf("an array of %d values", len(vals)))
		}
		for i := range vals {
			raw, name := any(nil), fmt.Sprintf("value%d", i+1)
			if isArray {
				raw = xs[i]
			} else if r, present := obj[name]; present {
				raw = r
			} else {
				return nil, fmt.Errorf("%s: missing field %s", joinPath(path, name), name)
			}
			v, err := in.decodeWire(raw, arg(i), snake, env, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			vals[i] = v
		}
		return &Tuple{Elems: vals}, nil
	case "()":
		if x == nil {
			return ir.Unit{}, nil
		}
		return fail("null")
	case "List", "Set":
		xs, ok := x.([]any)
		if !ok {
			return fail("an array")
		}
		vals := make([]Value, len(xs))
		for i, e := range xs {
			v, err := in.decodeWire(e, arg(0), snake, env, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			vals[i] = v
		}
		if t.Name == "Set" {
			return newSet(vals), nil
		}
		return &List{Elems: vals}, nil
	case "Map":
		if entries, ok := x.([]any); ok {
			return in.decodeEntries(entries, arg(0), arg(1), snake, env, path)
		}
		obj, ok := x.(map[string]any)
		if !ok {
			return fail("an object")
		}
		var keys, vals []Value
		for _, k := range sortedKeys(obj) {
			key, err := in.decodeWire(k, arg(0), snake, env, path)
			if err != nil {
				return nil, err
			}
			v, err := in.decodeWire(obj[k], arg(1), snake, env, joinPath(path, k))
			if err != nil {
				return nil, err
			}
			keys, vals = append(keys, key), append(vals, v)
		}
		return newMap(keys, vals), nil
	case "record":
		obj, ok := x.(map[string]any)
		if !ok {
			return fail("an object")
		}
		rec := &Record{}
		for i, name := range t.Fields {
			field := env.resolve(t.Args[i])
			raw, present := obj[wireName(name, snake)]
			if !present {
				if field.Decl != nil && in.isOption(field.Decl) {
					rec.Names = append(rec.Names, name)
					rec.Vals = append(rec.Vals, in.none())
					continue
				}
				return nil, fmt.Errorf("%s: missing field %s", strings.TrimPrefix(joinPath(path, wireName(name, snake)), "."), wireName(name, snake))
			}
			v, err := in.decodeWire(raw, field, snake, env, joinPath(path, wireName(name, snake)))
			if err != nil {
				return nil, err
			}
			rec.Names = append(rec.Names, name)
			rec.Vals = append(rec.Vals, v)
		}
		return rec, nil
	}
	return fail(t.Name)
}

// timeOfDayPattern is how a model writes a Time: 14:30 or 14:30:15.
const timeOfDayPattern = `^([01][0-9]|2[0-3]):[0-5][0-9](:[0-5][0-9])?$`

var timeOfDayRE = regexp.MustCompile(timeOfDayPattern)

// decodeEntries reads a map written as a list of entries: pairs, as toWire
// writes a map whose keys aren't strings, or objects with a key and a value,
// as schemaOf asks a model for a map.
func (in *Interp) decodeEntries(entries []any, keyType, valType *ir.TypeExpr, snake bool, env typeEnv, path string) (Value, error) {
	m := &Map{}
	for i, e := range entries {
		at := fmt.Sprintf("%s[%d]", path, i)
		var rawKey, rawVal any
		switch x := e.(type) {
		case []any:
			if len(x) != 2 {
				return nil, fmt.Errorf("%s: expected a key and a value, found %s", at, describeJSON(e))
			}
			rawKey, rawVal = x[0], x[1]
		case map[string]any:
			k, hasKey := x["key"]
			v, hasVal := x["value"]
			if !hasKey || !hasVal {
				return nil, fmt.Errorf("%s: expected an object with a key and a value, found %s", at, describeJSON(e))
			}
			rawKey, rawVal = k, v
		default:
			return nil, fmt.Errorf("%s: expected an entry with a key and a value, found %s", at, describeJSON(e))
		}
		key, err := in.decodeWire(rawKey, keyType, snake, env, joinPath(at, "key"))
		if err != nil {
			return nil, err
		}
		val, err := in.decodeWire(rawVal, valType, snake, env, joinPath(at, "value"))
		if err != nil {
			return nil, err
		}
		m = m.with(key, val)
	}
	return m, nil
}

func joinPath(path, field string) string {
	if path == "" {
		return field
	}
	return path + "." + field
}

func (in *Interp) decodeDecl(x any, d *ir.Type, env typeEnv, snake bool, path string) (Value, error) {
	if d.Alias != nil && len(d.Variants) == 0 {
		v, err := in.decodeWire(x, d.Alias, snake, env, path)
		if r, ok := v.(*Record); ok && d.Alias.Name == "record" {
			r.Type = d
		}
		return v, err
	}
	switch {
	case in.isOption(d):
		if x == nil {
			return in.none(), nil
		}
		v, err := in.decodeWire(x, d.Variants[0].Fields[0].Type, snake, env, path)
		if err != nil {
			return nil, err
		}
		return in.some(v), nil
	case d.Newtype():
		v, err := in.decodeWire(x, d.Variants[0].Fields[0].Type, snake, env, path)
		if err != nil {
			return nil, err
		}
		return &Variant{V: d.Variants[0], Vals: []Value{v}}, nil
	case isEnum(d):
		if s, ok := x.(string); ok {
			for _, v := range d.Variants {
				if wireName(v.Name, snake) == s {
					return &Variant{V: v}, nil
				}
			}
		}
		var names []string
		for _, v := range d.Variants {
			names = append(names, fmt.Sprintf("%q", wireName(v.Name, snake)))
		}
		return nil, fmt.Errorf("%sexpected one of %s, found %s", prefix(path), strings.Join(names, ", "), describeJSON(x))
	}
	obj, ok := x.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%sexpected an object with a kind, found %s", prefix(path), describeJSON(x))
	}
	kind, _ := obj["kind"].(string)
	for _, v := range d.Variants {
		if v.Name == kind {
			return in.decodeFields(obj, v, env, snake, path)
		}
	}
	return nil, fmt.Errorf("%s%s has no kind %q", prefix(path), d.Name, kind)
}

// decodeFields reads a variant's fields from an object.
func (in *Interp) decodeFields(obj map[string]any, v *ir.Variant, env typeEnv, snake bool, path string) (Value, error) {
	vals := make([]Value, len(v.Fields))
	for i, f := range v.Fields {
		name := wireName(fieldName(v, i, len(v.Fields)), snake)
		raw, present := obj[name]
		ft := env.resolve(f.Type)
		if !present {
			if ft.Decl != nil && in.isOption(ft.Decl) {
				vals[i] = in.none()
				continue
			}
			return nil, fmt.Errorf("%smissing field %s", prefix(path), name)
		}
		val, err := in.decodeWire(raw, ft, snake, env, joinPath(path, name))
		if err != nil {
			return nil, err
		}
		vals[i] = val
	}
	return &Variant{V: v, Vals: vals}, nil
}

func prefix(path string) string {
	if path == "" {
		return ""
	}
	return path + ": "
}

// schemaOf is the JSON Schema of a type's JSON shape, with `##` comments as
// descriptions. It follows the strict subset model providers accept: every
// field is required, an Option allows null, and no other fields are allowed.
func (in *Interp) schemaOf(t *ir.TypeExpr, env typeEnv, seen map[*ir.Type]bool) (map[string]any, error) {
	t = env.resolve(t)
	if t == nil {
		return nil, fmt.Errorf("no type to ask for")
	}
	if d := t.Decl; d != nil {
		if seen[d] {
			return nil, fmt.Errorf("%s refers to itself, which a schema can't say", d.Name)
		}
		seen[d] = true
		defer delete(seen, d)
		s, err := in.declSchema(d, bindParams(d, t.Args, env), seen)
		if err == nil && d.Doc != "" {
			s["description"] = d.Doc
		}
		return s, err
	}
	switch t.Name {
	case "String", "Key", "Zone":
		return map[string]any{"type": "string"}, nil
	case "Int", "Duration", "Size":
		return map[string]any{"type": "integer"}, nil
	case "Time":
		return map[string]any{"type": "string", "pattern": timeOfDayPattern}, nil
	case "Float":
		return map[string]any{"type": "number"}, nil
	case "Bool":
		return map[string]any{"type": "boolean"}, nil
	case "Instant":
		return map[string]any{"type": "string", "format": "date-time"}, nil
	case "Date":
		return map[string]any{"type": "string", "format": "date"}, nil
	case "List", "Set":
		if len(t.Args) != 1 {
			return nil, fmt.Errorf("%s needs its element type", t.Name)
		}
		items, err := in.schemaOf(t.Args[0], env, seen)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": items}, nil
	case "Map":
		// Providers' strict schemas name every property of an object, so a
		// map is asked for as a list of entries.
		if len(t.Args) != 2 {
			return nil, fmt.Errorf("Map needs its key and value types")
		}
		entry, err := in.objectSchema([]string{"key", "value"}, t.Args, nil, env, seen)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": entry}, nil
	case "tuple":
		// A tuple is an object with fields value1, value2, …, as a variant's
		// positional fields are.
		names := make([]string, len(t.Args))
		for i := range names {
			names[i] = fmt.Sprintf("value%d", i+1)
		}
		return in.objectSchema(names, t.Args, nil, env, seen)
	case "record":
		return in.objectSchema(t.Fields, t.Args, nil, env, seen)
	}
	return nil, fmt.Errorf("%s has no schema: a model can be asked for records, variants, tuples, lists, sets, maps, strings, numbers, booleans, dates and times", t.Name)
}

func (in *Interp) declSchema(d *ir.Type, env typeEnv, seen map[*ir.Type]bool) (map[string]any, error) {
	if d.Alias != nil && len(d.Variants) == 0 {
		if d.Alias.Name == "record" {
			return in.objectSchema(d.Alias.Fields, d.Alias.Args, d.FieldDocs, env, seen)
		}
		return in.schemaOf(d.Alias, env, seen)
	}
	switch {
	case in.isOption(d):
		inner, err := in.schemaOf(d.Variants[0].Fields[0].Type, env, seen)
		if err != nil {
			return nil, err
		}
		return map[string]any{"anyOf": []any{inner, map[string]any{"type": "null"}}}, nil
	case d.Newtype():
		return in.schemaOf(d.Variants[0].Fields[0].Type, env, seen)
	case isEnum(d):
		var names []any
		for _, v := range d.Variants {
			names = append(names, v.Name)
		}
		return map[string]any{"type": "string", "enum": names}, nil
	}
	var options []any
	for _, v := range d.Variants {
		props := map[string]any{"kind": map[string]any{"type": "string", "const": v.Name}}
		required := []any{"kind"}
		for i, f := range v.Fields {
			name := fieldName(v, i, len(v.Fields))
			s, err := in.schemaOf(f.Type, env, seen)
			if err != nil {
				return nil, err
			}
			props[name] = s
			required = append(required, name)
		}
		o := map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
		if v.Doc != "" {
			o["description"] = v.Doc
		}
		options = append(options, o)
	}
	return map[string]any{"anyOf": options}, nil
}

func (in *Interp) objectSchema(fields []string, types []*ir.TypeExpr, docs map[string]string, env typeEnv, seen map[*ir.Type]bool) (map[string]any, error) {
	props := map[string]any{}
	required := []any{}
	for i, name := range fields {
		s, err := in.schemaOf(types[i], env, seen)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", name, err)
		}
		if doc := docs[name]; doc != "" {
			s["description"] = doc
		}
		props[name] = s
		required = append(required, name)
	}
	sort.Slice(required, func(i, j int) bool { return required[i].(string) < required[j].(string) })
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}, nil
}
