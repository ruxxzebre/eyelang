package eval

import (
	"eyelang/object"
	"math"
)

var MathLib = builtinMapToScope("math", Math)

var Math = map[string]*object.Builtin{
	"pow": {
		Fn: func(env *object.Environment, args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("unexpected args. want=2. got=%d", len(args))
			}

			num, ok := args[0].(*object.Integer)
			if !ok {
				return newError("expected base to be %s. got=%s", num.Type(), args[0].Type())
			}

			pow, ok := args[1].(*object.Integer)
			if !ok {
				return newError("expected pow to be %s. got=%s", num.Type(), args[0].Type())
			}

			val := int64(math.Pow(float64(num.Value), float64(pow.Value)))
			return &object.Integer{Value: val}
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
