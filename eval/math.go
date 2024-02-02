package eval

import (
	"eyelang/object"
	"math"
)

var MathLib = builtinMapToScope("math", Math)

var Math = map[string]*object.Builtin{
	"pow": {
		Fn: func(env *object.Environment, args ...object.Object) object.Object {
			if len(args) == 0 || len(args) > 2 {
				return newError("unexpected args")
			}

			if num, ok := args[0].(*object.Integer); ok {
				if pow, ok := args[1].(*object.Integer); ok && len(args) == 2 {
					return &object.Integer{Value: int64(math.Pow(float64(num.Value), float64(pow.Value)))}
				} else if len(args) == 1 {
					return &object.Integer{Value: int64(math.Pow(float64(num.Value), 2))}
				}
				return newError("expected args[1] to be Integer. got=%T", args[1])
			}
			return newError("expected args[0] to be Integer. got=%T", args[1])
		},
	},
	"sqrt": {
		Fn: func(env *object.Environment, args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("unexpected args")
			}

			if num, ok := args[0].(*object.Integer); ok {
				return &object.Integer{int64(math.Sqrt(float64(num.Value)))}
			}
			return newError("expected args[0] to be Integer. got=%T", args[1])
		},
	},
}
