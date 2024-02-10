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
			if args[0].Type() != args[1].Type() {
				return newError("expected args to be the same type. got=%s and %s", args[0].Type(), args[1].Type())
			}

			switch args[0].(type) {
			case *object.Integer:
				num1 := args[0].(*object.Integer)
				num2 := args[1].(*object.Integer)
				val := int64(math.Pow(float64(num1.Value), float64(num2.Value)))
				return &object.Integer{Value: val}
			case *object.Float:
				num1 := args[0].(*object.Float)
				num2 := args[1].(*object.Float)
				val := math.Pow(num1.Value, num2.Value)
				return &object.Float{Value: val}
			default:
				return newError("expected base to be %s. got=%s", args[0].Type(), args[0].Type())
			}
		},
	},
	"sqrt": {
		Fn: func(env *object.Environment, args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("unexpected args")
			}

			if args[0].Type() == object.INTEGER_OBJ {
				num := args[0].(*object.Integer)
				val := int64(math.Sqrt(float64(num.Value)))
				return &object.Integer{Value: val}
			}

			if args[0].Type() == object.FLOAT_OBJ {
				num := args[0].(*object.Float)
				val := math.Sqrt(num.Value)
				return &object.Float{Value: val}
			}

			return newError("expected args[0] to be Integer or Float. got=%s", args[0].Type())
		},
	},
}
