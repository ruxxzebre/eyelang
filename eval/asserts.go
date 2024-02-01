package eval

import "eyelang/object"

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
