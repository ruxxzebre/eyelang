package object

import (
	"bytes"
	"eyelang/ast"
	"fmt"
	"hash/fnv"
	"strings"
)

type ObjectType string

// Types
const (
	INTEGER_OBJ      = "INTEGER"
	FLOAT_OBJ        = "FLOAT"
	BOOLEAN_OBJ      = "BOOLEAN"
	NULL_OBJ         = "NULL"
	RETURN_VALUE_OBJ = "RETURN_VALUE"
	ERROR_OBJ        = "ERROR"
	FUNCTION_OBJ     = "FUNCTION"
	STRING_OBJ       = "STRING"
	BUILTIN_OBJ      = "BUILTIN"
	ARRAY_OBJ        = "ARRAY"
	MACRO_OBJ        = "MACRO"
	HASH_OBJ         = "HASH"
	SCOPE_OBJ        = "SCOPE" // Module system
)

type HashKey struct {
	Type  ObjectType
	Value uint64
}

type Object interface {
	Type() ObjectType
	Inspect() string
}

type Hashable interface {
	HashKey() HashKey
}

// TODO:
// Should it be cleared in separate routine? (ref counting?)
// why it should be more performant btw???
var hashKeyCache = map[ObjectType]map[uint64]HashKey{}

func cacheHK(key uint64, typ ObjectType) HashKey {
	if hasType, ok := hashKeyCache[typ]; ok {
		if hasKey, ok := hasType[key]; ok {
			return hasKey
		}
	}
	if hashKeyCache[typ] == nil {
		hashKeyCache[typ] = map[uint64]HashKey{}
	}

	hk := HashKey{Type: typ, Value: key}
	hashKeyCache[typ][key] = hk
	return hk
}

type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return e.Message }

type Integer struct {
	Value int64
}

func (i *Integer) Inspect() string { return fmt.Sprintf("%d", i.Value) }

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }

func (i *Integer) HashKey() HashKey {
	// TODO: wtf if less then 0 btw
	return cacheHK(uint64(i.Value), INTEGER_OBJ)
}

type Float struct {
	Value float64
}

func (f *Float) Inspect() string { return strings.TrimRight(fmt.Sprintf("%f", f.Value), "0") }

func (f *Float) Type() ObjectType { return FLOAT_OBJ }

func (f *Float) HashKey() HashKey {
	// TODO: collisions over here!
	return cacheHK(uint64(f.Value), FLOAT_OBJ)
}

type Boolean struct {
	Value bool
}

func (b *Boolean) Inspect() string { return fmt.Sprintf("%t", b.Value) }

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }

func (b *Boolean) HashKey() HashKey {
	var val uint64 = 0
	if b.Value {
		val = 1
	}

	return cacheHK(val, BOOLEAN_OBJ)
}

type Null struct{}

func (n *Null) Inspect() string { return "null" }

func (n *Null) Type() ObjectType { return NULL_OBJ }

type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Inspect() string { return rv.Value.Inspect() }

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }

type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (fn *Function) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range fn.Parameters {
		params = append(params, p.String())
	}

	body := fn.Body.String()
	if body == "" {
		out.WriteString("fn(" + strings.Join(params, ", ") + ") {}")
	} else {
		out.WriteString("fn(" + strings.Join(params, ", ") + ") {\n")
		out.WriteString("\t" + body)
		out.WriteString("\n}")
	}

	return out.String()
}

func (fn *Function) Type() ObjectType { return FUNCTION_OBJ }

type String struct {
	Value string
}

func (st *String) Inspect() string { return st.Value }

func (st *String) Type() ObjectType { return STRING_OBJ }

func (st *String) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(st.Value))

	return cacheHK(h.Sum64(), STRING_OBJ)
}

type BuiltinFunction func(env *Environment, args ...Object) Object

type BultinParam struct {
	Type     ObjectType
	Name     string
	Required bool
}

type Builtin struct {
	Fn          BuiltinFunction
	Description string
	Parameters  []BultinParam
}

func (b *Builtin) Inspect() string { return "builtin function" }

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }

// type MacroFunction func(env *Environment, evaluator func(node ast.Node, env *Environment) Object) Object
type Macro struct {
	Name string
}

func (m *Macro) Inspect() string  { return fmt.Sprintf("[macro::%s]", m.Name) }
func (m *Macro) Type() ObjectType { return MACRO_OBJ }

type Array struct {
	Elements []Object
}

func (a *Array) Inspect() string {
	elements := []string{}
	for _, el := range a.Elements {
		elements = append(elements, el.Inspect())
	}

	var out bytes.Buffer

	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")

	return out.String()
}

func (a *Array) Type() ObjectType { return ARRAY_OBJ }

type HashPair struct {
	Key   Object
	Value Object
}

type Hash struct {
	Pairs map[HashKey]HashPair
}

func (h *Hash) Type() ObjectType { return HASH_OBJ }

func (h *Hash) Inspect() string {
	var out bytes.Buffer

	pairs := []string{}

	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s", pair.Key.Inspect(), pair.Value.Inspect()))
	}

	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}

type Scope struct {
	Outer *Environment // no that's bullshit
	Name  string
	// TODO: consider scope issues of imported functions (fix?)
	// Upd: fixed
	// Upd: separate scope and environment
	// What is the scope?
	// What is an environment?
}

func (s *Scope) Type() ObjectType { return SCOPE_OBJ }
func (s *Scope) Inspect() string {
	// nil scope
	if s.Outer.store == nil {
		return "nil scope" // TODO
	}

	if len(s.Outer.store) == 0 {
		return fmt.Sprintf("[scope::%s]", s.Name)
	}

	PER_ROW := 3
	count := 0

	var out bytes.Buffer
	out.WriteString(fmt.Sprintf("[scope::%s] -> %s", s.Name, "\n"))

	for key := range s.Outer.store {
		count++

		if count%PER_ROW == 0 {
			count = 0
			out.WriteString(key + " \n")
		} else {
			out.WriteString(key + " ")
		}
	}

	return out.String()
}
