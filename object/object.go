package object

import (
	"bytes"
	"eyelang/ast"
	"fmt"
	"strings"
)

type ObjectType string

// Types
const (
	INTEGER_OBJ      = "INTEGER"
	BOOLEAN_OBJ      = "BOOLEAN"
	NULL_OBJ         = "NULL"
	RETURN_VALUE_OBJ = "RETURN_VALUE"
	ERROR_OBJ        = "ERROR"
	FUNCTION_OBJ     = "FUNCTION"
	STRING_OBJ       = "STRING"
	BUILTIN_OBJ      = "BUILTIN"
	ARRAY_OBJ        = "ARRAY"
	MACRO_OBJ        = "MACRO"
)

type Object interface {
	Type() ObjectType
	Inspect() string
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

type Boolean struct {
	Value bool
}

func (b *Boolean) Inspect() string { return fmt.Sprintf("%t", b.Value) }

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }

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

	out.WriteString("fn(" + strings.Join(params, ", ") + ") {\n")
	out.WriteString("\t" + fn.Body.String())
	out.WriteString("\n}")

	return out.String()
}

func (fn *Function) Type() ObjectType { return FUNCTION_OBJ }

type String struct {
	Value string
}

func (st *String) Inspect() string { return st.Value }

func (st *String) Type() ObjectType { return STRING_OBJ }

type BuiltinFunction func(env *Environment, args ...Object) Object

type Builtin struct {
	Fn BuiltinFunction
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
