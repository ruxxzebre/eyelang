package eval

import "eyelang/object"

func builtinMapToScope(name string, lib map[string]*object.Builtin) *object.Scope {
	// TODO: debugging scopes/envs will be HARD (maybe)
	scope := &object.Scope{Outer: object.NewEnvironment(), Name: name}
	for name, builtin := range lib {
		scope.Outer.Set(name, builtin)
	}

	return scope
}

var HashLib = builtinMapToScope("noop", map[string]*object.Builtin{})

var ListsLib = builtinMapToScope("lists", Lists)

var Lists = map[string]*object.Builtin{
	"first": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if err := assertBuiltinArgsLength(args, "==", 1); err != nil {
				return err
			}

			if err := assertBuiltinArgType(args[0], object.ARRAY_OBJ, "first"); err != nil {
				return err
			}

			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length == 0 {
				return newError("array is empty")
			}

			return arr.Elements[0]
		},
	},
	"last": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if err := assertBuiltinArgsLength(args, "==", 1); err != nil {
				return err
			}

			if err := assertBuiltinArgType(args[0], object.ARRAY_OBJ, "last"); err != nil {
				return err
			}

			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length == 0 {
				return newError("array is empty")
			}

			return arr.Elements[length-1]
		},
	},
	"rest": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if err := assertBuiltinArgsLength(args, "==", 1); err != nil {
				return err
			}

			if err := assertBuiltinArgType(args[0], object.ARRAY_OBJ, "rest"); err != nil {
				return err
			}

			arr := args[0].(*object.Array)

			length := len(arr.Elements)
			if length == 0 {
				return newError("array is empty")
			}

			newArr := copyArray(arr.Elements[1:])

			return &object.Array{
				Elements: newArr,
			}
		},
	},
	"push": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if err := assertBuiltinArgsLength(args, ">", 2); err != nil {
				return err
			}

			if err := assertBuiltinArgType(args[0], object.ARRAY_OBJ, "last"); err != nil {
				return err
			}

			arr := args[0].(*object.Array)

			newArr := copyArray(arr.Elements)
			newArr = append(newArr, args[1:]...)

			return &object.Array{
				Elements: newArr,
			}
		},
	},
}
