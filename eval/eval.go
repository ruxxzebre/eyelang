package eval

import (
	"eyelang/ast"
	"eyelang/lexer"
	"eyelang/object"
	"eyelang/parser"
	"fmt"
	"go/token"
	"os"
)

var (
	TRUE       = &object.Boolean{Value: true}
	FALSE      = &object.Boolean{Value: false}
	NULL       = &object.Null{}
	LOAD_MACRO = &object.Macro{Name: "load"}
)

type EvalError struct {
	Token   token.Token
	Message string
}

var errorContext []EvalError

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func loadFromString(file string, env *object.Environment) {
	l := lexer.New(string(file))
	p := parser.New(l)
	program := p.ParseProgram()

	Eval(program, env)
}

func isMacro(obj object.Object) bool {
	return obj == LOAD_MACRO
}

func evalMacro(evaluation object.Object, args []object.Object, env *object.Environment) object.Object {
	switch evaluation {
	case LOAD_MACRO:
		filepath := args[0].(*object.String).Value
		file, err := os.ReadFile(filepath)
		if err != nil {
			return newError("file not found")
		}
		loadFromString(string(file), env)
		return evaluation
	}

	return newError("macro not found")
}

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(node, env)
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)
	case *ast.CallExpression:
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}

		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}

		return applyFunction(function, args, env)
	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Body: body, Env: env}
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)
	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}

		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}

		return evalInfixExpression(node.Operator, right, left)
	case *ast.IfExpression:
		return evalIfExpression(node, env)
	case *ast.BlockStatement:
		return evalBlockStatement(node, env)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.LetStatement:
		val := evalLetStatement(node, env)
		if isError(val) {
			return val
		}

		env.Set(node.Name.Value, val)
		return val
	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}
	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}
	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}

		right := Eval(node.Index, env)
		if isError(right) {
			return right
		}

		return evalIndexExpression(left, right)
	case *ast.HashLiteral:
		hash := &object.Hash{}
		pairs := map[object.HashKey]object.HashPair{}

		for key, value := range node.Pairs {
			left := Eval(key, env)
			if isError(left) {
				return left
			}

			right := Eval(value, env)
			if isError(right) {
				return right
			}

			hashed, ok := left.(object.Hashable)
			if !ok {
				return newError("unsupported hash key type: %T", left)
			}

			pairs[hashed.HashKey()] = object.HashPair{
				Key:   left,
				Value: right,
			}
		}

		hash.Pairs = pairs
		return hash
	}

	return newError("invalid type")
}

func evalProgram(node *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range node.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}

	return result
}

// func isInt(obj object.Object) *object.Integer {
// 	if node, ok := obj.(*object.Integer); ok {
// 		return node
// 	}
// 	return nil
// }

func isTruthy(obj object.Object) bool {
	// if node, ok := obj.(*object.Integer); ok {
	// 	if node.Value == 0 {
	// 		return false
	// 	}
	// 	return true
	// }

	switch obj {
	case TRUE:
		return true
	case FALSE:
		return false
	case NULL:
		return false
	default:
		return true
	}
}

func evalIfExpression(ie *ast.IfExpression, env *object.Environment) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	}

	return NULL
}

func evalStatements(statements []ast.Statement, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range statements {
		result = Eval(statement, env)
	}

	return result
}

func evalInfixExpression(operator string, right, left object.Object) object.Object {
	switch {
	case right.Type() != left.Type():
		return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right)
	// Comparing TRUE and FALSE objects
	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIntegerInfixExpression(operator string, right object.Object, left object.Object) object.Object {
	rightVal := right.(*object.Integer).Value
	leftVal := left.(*object.Integer).Value

	switch operator {
	case "-":
		return &object.Integer{Value: rightVal - leftVal}
	case "*":
		return &object.Integer{Value: rightVal * leftVal}
	case "+":
		return &object.Integer{Value: rightVal + leftVal}
	case "/":
		return &object.Integer{Value: rightVal / leftVal}
	case "<":
		return nativeBoolToBooleanObject(rightVal < leftVal)
	case ">":
		return nativeBoolToBooleanObject(rightVal > leftVal)
	case "==":
		return nativeBoolToBooleanObject(rightVal == leftVal)
	case "!=":
		return nativeBoolToBooleanObject(rightVal != leftVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalStringInfixExpression(operator string, right, left object.Object) object.Object {
	rightVal := right.(*object.String).Value
	leftVal := left.(*object.String).Value

	switch operator {
	case "+":
		return &object.String{Value: rightVal + leftVal}
	case "==":
		return nativeBoolToBooleanObject(rightVal == leftVal)
	case "!=":
		return nativeBoolToBooleanObject(rightVal != leftVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusOperatorExpression(right)
	default:
		// return NULL
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
	if isTruthy(right) {
		return FALSE
	}
	return TRUE
}

func evalMinusOperatorExpression(right object.Object) object.Object {
	if right.Type() != object.INTEGER_OBJ {
		return newError("unknown operator: -%s", right.Type())
	}

	value := right.(*object.Integer).Value
	return &object.Integer{Value: -value}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}

	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}

	return newError("identifier not found: %s", node.Value)
}

func evalBlockStatement(node *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range node.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}

	return result
}

func evalLetStatement(node *ast.LetStatement, env *object.Environment) object.Object {
	return Eval(node.Value, env)
}

func evalExpressions(nodes []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object
	for _, e := range nodes {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

func extendFunctionEnv(fn *object.Function, args []object.Object) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	// TODO: test for that case
	// !!! args should be the same length as fn.Parameters, or we're getting PANIC
	// but this should be checked (what: in the parser) outside of extendFunctionEnv
	if len(args) == 0 {
		return env
	}

	for idx, param := range fn.Parameters {
		env.Set(param.Value, args[idx])
	}

	return env
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

func applyFunction(fn object.Object, args []object.Object, env *object.Environment) object.Object {
	switch fn := fn.(type) {
	case *object.Builtin:
		result := fn.Fn(env, args...)

		if isMacro(result) {
			return evalMacro(result, args, env)
		}

		return result

	case *object.Function:
		if len(fn.Parameters) != len(args) {
			return newError("wrong number of arguments. expected=%d got=%d"+
				"\nIf you wanted to curry the function, it's not supported yet",
				len(fn.Parameters), len(args))
		}

		extendedEnv := extendFunctionEnv(fn, args)
		evaludated := Eval(fn.Body, extendedEnv)
		return unwrapReturnValue(evaludated)
	default:
		return newError("not a function: %s", fn.Type())
	}
}

func evalIndexExpression(left, right object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, right)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(left, right)
	default:
		return newError("index operator not supported for type: %s", left.Type())
	}
}

func evalHashIndexExpression(left, right object.Object) object.Object {
	hashable, ok := right.(object.Hashable)
	if ok {
		hash := left.(*object.Hash)
		if val, ok := hash.Pairs[hashable.HashKey()]; ok {
			return val.Value
		}
		return NULL
	}
	return newError("hash index cannot be %s", right.Type())
}

func evalArrayIndexExpression(left, right object.Object) object.Object {
	arr := left.(*object.Array)
	idx := right.(*object.Integer).Value
	max := int64(len(arr.Elements) - 1)

	if idx > max {
		return newError("index above array limit. at least=%d. got=%d", len(arr.Elements), int(idx))
	}

	if idx < 0 {
		return newError("index cannot be < 0. got=%d", idx)
	}

	return arr.Elements[idx]
}
