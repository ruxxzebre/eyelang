package eval

import (
	"eyelang/object"
	"fmt"
	"os"
	"strings"
)

func assertBuiltinArgsLength(args []object.Object, op string, want int) object.Object {
	switch op {
	case ">":
		if len(args) < want {
			return newError("wrong number of arguments. expected at least %d. got=%d", want, len(args))
		}
	case "<":
		if len(args) > want {
			return newError("wrong number of arguments. expected less than %d. got=%d", want, len(args))
		}
	case "==":
		if len(args) != want {
			return newError("wrong number of arguments. expected=%d. got=%d", want, len(args))
		}
	}
	return nil
}

func assertBuiltinArray(arr object.Array) object.Object {
	panic("not implemented")
}

func assertBuiltinArgType(obj object.Object, kind object.ObjectType, metadata string) object.Object {
	if obj.Type() != kind {
		return newError("argument is not supported by %s. expected %s. got=%s", metadata, kind, obj.Type())
	}
	return nil
}

func copyArray(arr []object.Object) []object.Object {
	newArr := make([]object.Object, len(arr), len(arr))
	copy(newArr, arr)
	return newArr
}

// func mergeArrays(arr1 []object.Object) // TODO

var builtins = map[string]*object.Builtin{
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
}
