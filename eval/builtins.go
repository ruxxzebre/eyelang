package eval

import (
	"eyelang/object"
	"fmt"
	"os"
	"strings"
)

func copyArray(arr []object.Object) []object.Object {
	newArr := make([]object.Object, len(arr), len(arr))
	copy(newArr, arr)
	return newArr
}

func loadLibIntoScope(lib map[string]*object.Builtin, scope *object.Scope) {
	for name, builtin := range lib {
		scope.Outer.Set(name, builtin)
	}
}

var Builtins = map[string]*object.Builtin{
	"len": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if err := assertBuiltinArgsLength(args, "==", 1); err != nil {
				return err
			}

			switch arg := args[0].(type) {
			case *object.String:
				return &object.Integer{Value: int64(len(arg.Value))}
			case *object.Array:
				return &object.Integer{Value: int64(len(arg.Elements))}
			default:
				return newError("argument to `len` is not supported. got=%s", args[0].Type())
			}
		},
	},
	"puts": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {

			elements := []string{}
			for _, arg := range args {
				elements = append(elements, arg.Inspect())
			}

			fmt.Println(strings.Join(elements, ", "))

			return NULL
		},
	},
	"printFile": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if err := assertBuiltinArgsLength(args, "==", 1); err != nil {
				return err
			}

			if err := assertBuiltinArgType(args[0], object.FUNCTION_OBJ, "load"); err != nil {
				return err
			}

			filepath := args[0].(*object.String).Value
			file, err := os.ReadFile(filepath)
			if err != nil {
				return newError(err.Error())
			}

			fmt.Println(file)

			// loadFromString(string(file), env)

			return NULL
		},
	},
	"load": {
		Fn: func(_ *object.Environment, _args ...object.Object) object.Object {
			return LOAD_MACRO
		},
	},
	"import": {
		Fn: func(_ *object.Environment, _args ...object.Object) object.Object {
			return newError("not implemented")
		},
	},
	"builtins2": {
		Fn: func(_ *object.Environment, _args ...object.Object) object.Object {

			builtinTable := `
	puts
	len first last rest push
	printFile
	load import
	builtins
			`

			return &object.String{Value: builtinTable}
		},
	},
	// TODO: error as a value and a type
	"error": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("generic error")
			}

			return newError(args[0].Inspect())
		},
		// generate description from the function signature/definition/comments
		Description: "error(message: object.String) -> object.Error",
	},
	"exit": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			// check if scope is REPL
			if len(args) != 0 {
				return newError("exit does not take any arguments")
			}

			os.Exit(0)
			return NULL
		},
	},
}
