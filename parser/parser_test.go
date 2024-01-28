package parser

import (
	// assrt "eyelang/assert"
	"eyelang/ast"
	"eyelang/lexer"
	"eyelang/token"
	"fmt"

	// "fmt"
	"testing"
)

func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}

func testLetStatement(t *testing.T, stmt ast.Statement, ident string) bool {
	if stmt.TokenLiteral() != "let" {
		t.Errorf("stmt.TokenLiteral is not 'let'. got=%q", stmt.TokenLiteral())
		return false
	}

	letStmt, ok := stmt.(*ast.LetStatement)
	if !ok {
		t.Errorf("stmt not *astStatement. got=%T", stmt)
		return false
	}

	if letStmt.Name.Value != ident {
		t.Errorf("letStmt.Name.Value not '%s'. got=%s", ident, letStmt.Name.TokenLiteral())
		return false
	}

	return true
}

func TestLetStatements(t *testing.T) {
	input := `
let x = 5;
let y = 10;

let foobar = 838383;
	`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if program == nil {
		t.Fatalf("Program cannot be nil")
	}

	if len(program.Statements) != 3 {
		t.Fatalf("Program should contain 3 statements, received %v", len(program.Statements))
	}

	tests := []struct {
		expectedIdentifier string
	}{
		{"x"},
		{"y"},
		{"foobar"},
	}

	for i, tt := range tests {
		stmt := program.Statements[i]
		if !testLetStatement(t, stmt, tt.expectedIdentifier) {
			return
		}
	}
}

func TestReturnStatements(t *testing.T) {
	input := `
return 5;
return x + y;
return flex;
	`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if program == nil {
		t.Fatalf("*ast.Program cannot be nil")
	}

	if len(program.Statements) != 3 {
		t.Fatalf("Program should contain 3 statements, received %v", len(program.Statements))
	}

	for _, stmt := range program.Statements {
		if stmt.TokenLiteral() != "return" {
			t.Fatalf("token is not 'return'")
		}
	}
}

func TestProgram(t *testing.T) {
	program := &ast.Program{
		Statements: []ast.Statement{
			&ast.LetStatement{
				Token: token.Token{Type: token.LET, Literal: "let"},
				Name: &ast.Identifier{
					Token: token.Token{Type: token.IDENT, Literal: "x"},
					Value: "x",
				},
				Value: &ast.Identifier{
					Token: token.Token{Type: token.INT, Literal: "21212"},
					Value: "21212",
				},
			},
		},
	}

	if program.String() != "let x = 21212;" {
		t.Errorf("program.String() wrong. got=%q", program.String())
	}
}

func TestIdentifierExpression(t *testing.T) {
	input := "foobar;"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
	}

	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	ident, ok := stmt.Expression.(*ast.Identifier)
	if !ok {
		t.Fatalf("exp not *ast.Identifier. got=%T", stmt.Expression)
	}
	if ident.Value != "foobar" {
		t.Errorf("ident.Value not %s. got=%s", "foobar", ident.Value)
	}
	if ident.TokenLiteral() != "foobar" {
		t.Errorf("ident.TokenLiteral not %s. got=%s", "foobar",
			ident.TokenLiteral())
	}
}

func TestIntegerLiteralExpression(t *testing.T) {
	input := "5;"
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(program.Statements) != 1 {
		t.Errorf("program has not enough statements. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not *ast.ExpressionStatement. got=%T", program.Statements[0])
	}

	literal, ok := stmt.Expression.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("exp not *ast.IntegerLiteral. got=%T", stmt.Expression)
	}
	if literal.Value != 5 {
		t.Errorf("literal.Value not=%d. got=%d", 5, literal.Value)
	}

	if literal.TokenLiteral() != "5" {
		t.Errorf("literal.TokenLiteral not %s. got=%s", "5", literal.TokenLiteral())
	}
}

func testIdentifier(t *testing.T, exp ast.Expression, value string) bool {
	ident, ok := exp.(*ast.Identifier)
	if !ok {
		t.Errorf("exp not *ast.Identifier. got=%T", exp)
		return false
	}

	if ident.Value != value {
		t.Errorf("expected indent to be %s. got=%s", value, ident.Value)
		return false
	}

	if ident.TokenLiteral() != value {
		t.Errorf("ident.TokenLiteral not %s. got=%s", value, ident.TokenLiteral())
		return false
	}

	return true
}

func testLiteralExpression(t *testing.T, exp ast.Expression, expected interface{}) bool {
	switch v := expected.(type) {
	case int:
		return testIntegerLiteral(t, exp, int64(v))
	case int64:
		return testIntegerLiteral(t, exp, v)
	case string:
		return testIdentifier(t, exp, v)
	case bool:
		return testBooleanLiteral(t, exp, v)
	}
	t.Errorf("type of exp is not handled. expected=%T. got=%T", expected, exp)
	return false
}

func testInfixExpression(t *testing.T, exp ast.Expression, left interface{}, operator string, right interface{}) bool {
	opExp, ok := exp.(*ast.InfixExpression)
	if !ok {
		t.Errorf("exp is not ast.InfixExpression. got=%T(%s)", exp, exp)
		return false
	}

	if !testLiteralExpression(t, opExp.Left, left) {
		return false
	}

	if opExp.Operator != operator {
		t.Errorf("exp.Operator is not '%s'. got=%q", operator, opExp.Operator)
		return false
	}

	if !testLiteralExpression(t, opExp.Right, right) {
		return false
	}

	return true
}

func testIntegerLiteral(t *testing.T, il ast.Expression, value int64) bool {
	integ, ok := il.(*ast.IntegerLiteral)
	if !ok {
		t.Errorf("il not *ast.IntegerLiteral. got=%T", il)
		return false
	}

	if integ.Value != value {
		t.Errorf("integ.Value is not %d. got=%d", value, integ.Value)
		return false
	}

	if integ.TokenLiteral() != fmt.Sprintf("%d", value) {
		t.Errorf("integ.TokenLiteral is not %d. got=%s", value, integ.TokenLiteral())
		return false
	}

	return true
}

func TestParsingPrefixExpressions(t *testing.T) {
	prefixTests := []struct {
		input    string
		operator string
		value    interface{}
	}{
		{"!5;", "!", 5},
		{"-15;", "-", 15},
		{"!true;", "!", true},
		{"!false;", "!", false},
	}

	for _, tt := range prefixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()

		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Errorf("program has not enough statements. got=%d", len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
		}

		exp, ok := stmt.Expression.(*ast.PrefixExpression)
		if !ok {
			t.Fatalf("stmt is not ast.PrefixExpression. got=%T", stmt.Expression)
		}
		if exp.Operator != tt.operator {
			t.Fatalf("exp.Operator is not %s. got=%s", tt.operator, exp.Operator)
		}
		if !testLiteralExpression(t, exp.Right, tt.value) {
			return
		}
	}
}

func assert(t *testing.T, args ...interface{}) interface{} {
	testType, ok := args[0].(string)
	if !ok {
		t.Fatalf("expected args[0] to be string (testType). got=%T", args[0])
	}

	switch testType {
	case "isInt":
		num, ok := args[1].(int)
		if !ok {
			t.Fatalf("expected args[1] to be int (targetLen). got=%T", args[1])
		}
		return num
	case "isProgram":
		program, ok := args[1].(*ast.Program)
		if !ok {
			t.Fatalf("expected args[1] to be *ast.Program. got=%T", args[1])
		}
		return program
	case "programStmtLenEq":
		program := assert(t, "isProgram", args[1]).(*ast.Program)
		targetLen := assert(t, "isInt", args[2]).(int)

		if len(program.Statements) != targetLen {
			t.Fatalf("program.Statements does not contain %d statements. got=%d\n", targetLen, len(program.Statements))
		}
	case "programStmtLenGt":
		program := assert(t, "isProgram", args[1]).(*ast.Program)
		targetLen := assert(t, "isInt", args[2]).(int)

		if len(program.Statements) < targetLen {
			t.Fatalf("program.Statements does not contain more than %d statements. got=%d\n", targetLen, len(program.Statements))
		}
	case "expectStmt2BExprStmt":
		program := assert(t, "isProgram", args[1]).(*ast.Program)
		idx := assert(t, "isInt", args[2]).(int)

		assert(t, "programStmtLenGt", args[1], 0)

		stmt, ok := program.Statements[idx].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("expected program.Statements[%d] to be ExpressionStatement. got=%T", idx, program.Statements[idx])
		}

		return stmt
	default:
		t.Fatalf("assert. invalid testType. got=%s", testType)
	}

	return nil
}

func testBooleanLiteral(t *testing.T, exp ast.Expression, value bool) bool {
	boolexp, ok := exp.(*ast.Boolean)
	if !ok {
		t.Errorf("exp not *ast.Boolean. got=%T", exp)
		return false
	}

	if boolexp.Value != value {
		t.Errorf("boolexp.Value is not %t. got=%t", value, boolexp.Value)
		return false
	}

	if boolexp.TokenLiteral() != fmt.Sprintf("%t", value) {
		t.Errorf("boolexp.TokenLiteral is not '%t'. got='%s'", value, boolexp.TokenLiteral())
		return false
	}

	return true
}

func TestBooleanExpression(t *testing.T) {
	input := `
		true;
		false;
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	boolTests := []struct {
		expected string
		value    bool
	}{
		{"true", true},
		{"false", false},
	}

	for idx, tt := range boolTests {
		booleanExpr, ok := program.Statements[idx].(*ast.ExpressionStatement).Expression.(*ast.Boolean)
		if !ok {
			t.Errorf("expected program.Statements[0].Expression to be %T. got=%T", ast.Boolean{}, booleanExpr)
		}

		if booleanExpr.Value != tt.value {
			t.Errorf("expected program.Statements[0].Expression.Boolean to be %t. got=%t", tt.value, booleanExpr.Value)
		}

		if booleanExpr.TokenLiteral() != tt.expected {
			t.Errorf("expected program.Statements[0].Expression.Boolean.TokenLiteral to be %s. got=%s", tt.expected, booleanExpr.TokenLiteral())
		}
	}
}

func TestParsingInfixExpressions(t *testing.T) {
	// unmuteTracer()
	infixTests := []struct {
		input      string
		leftValue  interface{}
		operator   string
		rightValue interface{}
	}{
		{"5 + 5;", 5, "+", 5},
		{"5 - 5;", 5, "-", 5},
		{"5 * 5;", 5, "*", 5},
		{"5 / 5;", 5, "/", 5},
		{"5 > 5;", 5, ">", 5},
		{"5 < 5;", 5, "<", 5},
		{"5 == 5;", 5, "==", 5},
		{"5 != 5;", 5, "!=", 5},
		{"true == true", true, "==", true},
		{"true != false", true, "!=", false},
		{"false == false", false, "==", false},
	}

	for _, tt := range infixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		stmt := assert(t, "expectStmt2BExprStmt", program, 0).(*ast.ExpressionStatement)
		testInfixExpression(t, stmt.Expression, tt.leftValue, tt.operator, tt.rightValue)
	}
}

func TestOperatorPrecedenceParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"-a * b",
			"((-a) * b)",
		},
		{
			"!-a",
			"(!(-a))",
		},
		{
			"a + b + c",
			"((a + b) + c)",
		},
		{
			"a + b - c",
			"((a + b) - c)",
		},
		{
			"a * b * c",
			"((a * b) * c)",
		},
		{
			"a * b / c",
			"((a * b) / c)",
		},
		{
			"a + b / c",
			"(a + (b / c))",
		},
		{
			"a + b * c + d / e - f",
			"(((a + (b * c)) + (d / e)) - f)",
		},
		{
			"3 + 4; -5 * 5",
			"(3 + 4)((-5) * 5)",
		},
		{
			"5 > 4 == 3 < 4",
			"((5 > 4) == (3 < 4))",
		},
		{
			"5 < 4 != 3 > 4",
			"((5 < 4) != (3 > 4))",
		},
		{
			"3 + 4 * 5 == 3 * 1 + 4 * 5",
			"((3 + (4 * 5)) == ((3 * 1) + (4 * 5)))",
		},
		{
			"1 + (2 + 3) + 4",
			"((1 + (2 + 3)) + 4)",
		},
		{
			"(5 + 5) * 2",
			"((5 + 5) * 2)",
		},
		{
			"2 / (5 + 5)",
			"(2 / (5 + 5))",
		},
		{
			"-(5 + 5)",
			"(-(5 + 5))",
		},
		{
			"!(true == true)",
			"(!(true == true))",
		},
		{
			"a + add(b * c) + d",
			"((a + add((b * c))) + d)",
		},
		{
			"add(a, b, 1, 2 * 3, 4 + 5, add(6, 7 * 8))",
			"add(a, b, 1, (2 * 3), (4 + 5), add(6, (7 * 8)))",
		},
		{
			"add(a + b + c * d / f + g)",
			"add((((a + b) + ((c * d) / f)) + g))",
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		actual := program.String()
		if actual != tt.expected {
			t.Errorf("expected=%q, got=%q", tt.expected, actual)
		}
	}
}

func TestIfStatement(t *testing.T) {
	input := "if (x < y) { x }"
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	assert(t, "programStmtLenEq", program, 1)
	stmt := assert(t, "expectStmt2BExprStmt", program, 0).(*ast.ExpressionStatement)
	exp, ok := stmt.Expression.(*ast.IfExpression)
	if !ok {
		t.Fatalf("stmt.Expression is not *ast.IfExpression. got=%T", exp)
	}

	if !testInfixExpression(t, exp.Condition, "x", "<", "y") {
		return
	}

	if len(exp.Consequence.Statements) != 1 {
		t.Errorf("exp.Consequence.Statements expected %d statements. got=%d", 1, len(exp.Consequence.Statements))
	}

	consequence, ok := exp.Consequence.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("exp.Consequence.Statements[0] to be *ast.ExpressionStatement. got=%T", consequence)
	}

	if !testIdentifier(t, consequence.Expression, "x") {
		return
	}

	if exp.Alternative != nil {
		t.Errorf("exp.Alternative.Statements was not nil. got=%+v", exp.Alternative)
	}
}

func TestIfElseStatement(t *testing.T) {
	input := "if (x < y) { x } else { y }"
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	assert(t, "programStmtLenEq", program, 1)
	stmt := assert(t, "expectStmt2BExprStmt", program, 0).(*ast.ExpressionStatement)
	exp, ok := stmt.Expression.(*ast.IfExpression)
	if !ok {
		t.Fatalf("stmt.Expression is not *ast.IfExpression. got=%T", exp)
	}

	if !testInfixExpression(t, exp.Condition, "x", "<", "y") {
		return
	}

	if len(exp.Consequence.Statements) != 1 {
		t.Errorf("exp.Consequence.Statements expected %d statements. got=%d", 1, len(exp.Consequence.Statements))
	}

	consequence, ok := exp.Consequence.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("exp.Consequence.Statements[0] to be *ast.ExpressionStatement. got=%T", consequence)
	}

	if !testIdentifier(t, consequence.Expression, "x") {
		return
	}

	alternative, ok := exp.Alternative.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("exp.Alternative.Statements[0] to be *ast.ExpressionStatement. got=%T", exp.Alternative.Statements[0])
	}

	if alternative.Expression.String() != "y" {
		t.Fatalf("alternative.Expression.TokenLiteral to be 'y'. got='%s'", alternative.Expression.String())
	}
}

func TestFunctionLiteralParsing(t *testing.T) {
	input := `fn(x, y) { x + y; }`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	assert(t, "programStmtLenEq", program, 1)
	stmt := assert(t, "expectStmt2BExprStmt", program, 0).(*ast.ExpressionStatement)

	function, ok := stmt.Expression.(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("stmt.Expression is not *ast.FunctionLiteral. got=%T", stmt.Expression)
	}

	if len(function.Parameters) != 2 {
		t.Fatalf("expected function to have 2 parameters. got=%d", len(function.Parameters))
	}

	testLiteralExpression(t, function.Parameters[0], "x")
	testLiteralExpression(t, function.Parameters[1], "y")

	if len(function.Body.Statements) != 1 {
		t.Fatalf("function.Body.Statements has not 1 statements. got=%d\n",
			len(function.Body.Statements))
	}

	bodyStmt, ok := function.Body.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("function body stmt is not ast.ExpressionStatement. got=%T",
			function.Body.Statements[0])
	}

	testInfixExpression(t, bodyStmt.Expression, "x", "+", "y")
}

func TestFunctionParameterParsing(t *testing.T) {
	tests := []struct {
		input          string
		expectedParams []string
	}{
		{input: "fn() {};", expectedParams: []string{}},
		{input: "fn(x) {};", expectedParams: []string{"x"}},
		{input: "fn(x, y, z) {};", expectedParams: []string{"x", "y", "z"}},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		checkParserErrors(t, p)

		program := p.ParseProgram()

		assert(t, "programStmtLenEq", program, 1)
		stmt := assert(t, "expectStmt2BExprStmt", program, 0).(*ast.ExpressionStatement)

		funcStmt, ok := stmt.Expression.(*ast.FunctionLiteral)
		if !ok {
			t.Errorf("expected stmt.Expression to be *ast.FunctionLiteral. got=%T", stmt.Expression)
		}

		if len(funcStmt.Parameters) != len(tt.expectedParams) {
			t.Errorf("expected to have %d params. got=%d", len(tt.expectedParams), len(funcStmt.Parameters))
		}

		for i, ident := range tt.expectedParams {
			testLiteralExpression(t, funcStmt.Parameters[i], ident)
		}

		// for idx, param := range funcStmt.Parameters {
		// 	if param.Value != tt.expectedParams[idx] {
		// 		t.Errorf("expected funcStmt.Parameters[%d] to be %s. got=%s", idx, tt.expectedParams[idx], param.Value)
		// 	}
		// }
	}
}

func TestCallExpression(t *testing.T) {
	input := "add(1, 2 * 3, 4 + 5);"

	l := lexer.New(input)
	p := New(l)
	checkParserErrors(t, p)

	program := p.ParseProgram()

	assert(t, "programStmtLenEq", program, 1)
	stmt := assert(t, "expectStmt2BExprStmt", program, 0).(*ast.ExpressionStatement)

	callExpr, ok := stmt.Expression.(*ast.CallExpression)
	if !ok {
		t.Errorf("expected stmt.Expression to be *ast.CallExpression. got=%T", callExpr)
	}

	if !testIdentifier(t, callExpr.Function, "add") {
		return
	}

	if len(callExpr.Arguments) != 3 {
		t.Errorf("expected callExpr.Arguments to have 3 args. got=%d", len(callExpr.Arguments))
	}

	testLiteralExpression(t, callExpr.Arguments[0], 1)
	testInfixExpression(t, callExpr.Arguments[1], 2, "*", 3)
	testInfixExpression(t, callExpr.Arguments[2], 4, "+", 5)
}
