package eval

import (
	"eyelang/lexer"
	"eyelang/object"
	"eyelang/parser"
	"fmt"
	"os"
	"strings"
	"time"
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
	"eval": {
		Fn: func(env *object.Environment, args ...object.Object) object.Object {
			if len(args) > 2 || len(args) < 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}

			code, ok := args[0].(*object.String)
			if !ok {
				return newError("argument to `eval` not supported, got %s", args[0].Type())
			}

			l := lexer.New(code.Value)
			l.Debug = true
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				return newError(p.Errors()[0].Message)
			}

			return Eval(program, env)
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
	"help": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("help requires one argument")
			}

			switch arg := args[0].(type) {
			case *object.String:
				return &object.String{Value: arg.Value}
			case *object.Builtin:
				if arg.Description == "" {
					return &object.String{Value: "no description available"}
				}
				return &object.String{Value: arg.Description}
			case *object.Function:
				parts := []string{}
				for _, p := range arg.Parameters {
					parts = append(parts, p.String())
				}
				return &object.String{Value: fmt.Sprintf("func (%s)", strings.Join(parts, ", "))}
			default:
				return newError("help requires an argument of type string, function, or builtin")
			}
		},
	},
	"exit": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			// TODO: check if scope is REPL

			if len(args) != 0 {
				return newError("exit does not take any arguments")
			}

			os.Exit(0)
			return NULL
		},
	},
	"str": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("str requires an argument")
			}

			num, ok := args[0].(*object.Integer)
			if !ok {
				return newError("expected integer. got=%T", args[0])
			}

			return &object.String{
				Value: fmt.Sprintf("%d", num.Value),
			}
		},
	},
	"update!": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("expected 3 args")
			}

			switch arg := args[0].(type) {
			case *object.Array:
				idx, ok := args[1].(*object.Integer)
				if !ok {
					return newError("expected integer. got=%T", args[1])
				}
				if idx.Value < 0 || int(idx.Value) >= len(arg.Elements) {
					return newError("index out of range")
				}
				arg.Elements[idx.Value] = args[2]
				return arg
			case *object.Hash:
				key, ok := args[1].(object.Hashable)
				if !ok {
					return newError("unusable as hash key: %T", args[1])
				}
				arg.Pairs[key.HashKey()] = object.HashPair{Key: args[1], Value: args[2]}
				return arg
			default:
				return newError("argument to `update` is not supported. got=%s", args[0].Type())
			}
		},
	},
	"has": {
		Fn: func(_ *object.Environment, args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("expected 2 args")
			}

			switch arg := args[0].(type) {
			case *object.Array:
				for idx, el := range arg.Elements {
					if el.Inspect() == args[1].Inspect() {
						return &object.Integer{Value: int64(idx)}
					}
				}
				// TODO: Options
				return NULL
			case *object.String:
				if strings.Contains(arg.Value, args[1].Inspect()) {
					return TRUE
				}
				return FALSE
			case *object.Hash:
				hashable, ok := args[1].(object.Hashable)
				if !ok {
					return newError("unusable as hash key: %s", args[1].Type())
				}
				_, ok = arg.Pairs[hashable.HashKey()]
				if ok {
					return TRUE
				}
				return FALSE
			default:
				return newError("argument to `has` is not supported. got=%s", args[0].Type())
			}
		},
	},
	"time": {
		Fn: func(env *object.Environment, args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("expected 2 args")
			}

			nargs := args[1].(*object.Array)
			fn := args[0].(*object.Function)

			start := time.Now()
			result := applyFunction(fn, nargs.Elements, env)
			elapsed := time.Since(start)

			return &object.Array{
				Elements: []object.Object{
					&object.String{Value: fmt.Sprintf("executed in %s", elapsed)},
					result,
				},
			}
		},
	},
}
