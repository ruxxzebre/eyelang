package eval

import (
	"eyelang/object"
)

func InitReplScope() *object.Scope {
	env := object.NewEnvironment()

	scope := &object.Scope{Outer: env, Name: "repl"}
	env.Set("global", scope)
	env.Set("lists", ListsLib)
	env.Set("hash", HashLib)
	env.Set("math", MathLib)

	env.Set("scopeName", &object.String{Value: scope.Name})

	loadLibIntoScope(Builtins, scope)
	loadLibIntoScope(Lists, scope)

	return scope
}

// TODO!
func InitScriptScope() *object.Scope {
	env := object.NewEnvironment()

	scope := &object.Scope{Outer: env, Name: "script"}
	env.Set("global", scope)
	env.Set("lists", ListsLib)
	env.Set("hash", HashLib)

	env.Set("scopeName", &object.String{Value: scope.Name})

	loadLibIntoScope(Builtins, scope)
	loadLibIntoScope(Lists, scope)

	return scope
}

func initTestingScope() *object.Scope {
	env := object.NewEnvironment()

	scope := &object.Scope{Outer: env, Name: "testing"}
	env.Set("global", scope)
	env.Set("lists", ListsLib)
	env.Set("hash", HashLib)

	env.Set("scopeName", &object.String{Value: scope.Name})

	loadLibIntoScope(Builtins, scope)
	loadLibIntoScope(Lists, scope)

	return scope
}
