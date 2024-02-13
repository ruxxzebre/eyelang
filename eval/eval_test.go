package eval

import (
	"eyelang/lexer"
	"eyelang/object"
	"eyelang/parser"
	_ "eyelang/testing"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestEvalIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"5", 5},
		{"10", 10},
		{"-5", -5},
		{"-10", -10},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"-50 + 100 + -50", 0},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"20 + 2 * -10", 0},
		{"50 / 2 * 2 + 10", 60},
		{"2 * (5 + 10)", 30},
		{"3 * 3 * 3 + 10", 37},
		{"3 * (3 * 3) + 10", 37},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestEvalBooleanExpression(t *testing.T) {
	// defer parser.UnmuteTracer(parser.MuteTracer())
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"3 < 2", false},
		{"3 > 2", true},
		{"9 > 12", false},
		{"9 == 12", false},
		{"12 == 12", true},
		{"9 != 12", true},
		{"(1 + 3) > 0", true},
		{"(2 * 3) == 6", true},
		{"(2 / 3) != 6", true},
		{"(2 / 3) < 6", true},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func checkParserErrors(t *testing.T, p *parser.Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg.Message)
	}
	t.FailNow()
}

func testEval(input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		errors := []string{}
		for _, e := range p.Errors() {
			errors = append(errors, e.Message)
		}
		return newError("parser error: \n%s", strings.Join(errors, "\n"))
	}
	env := object.NewEnvironment()
	env.Load(initTestingScope().Outer)

	return Eval(program, env)
}

func testEval2(input string) (object.Object, *object.Environment) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	env.Load(initTestingScope().Outer)

	if len(p.Errors()) != 0 {
		for _, e := range p.Errors() {
			fmt.Println(e.Message)
		}
		return newError("parser error"), env
	}

	return Eval(program, env), env
}

func testEval3(input string, env *object.Environment) (object.Object, *object.Environment) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		for _, e := range p.Errors() {
			fmt.Println(e.Message)
		}
		return newError("parser error"), env
	}
	return Eval(program, env), env
}

func testBooleanObject(t *testing.T, obj object.Object, expected bool) bool {
	result, ok := obj.(*object.Boolean)
	if !ok {
		t.Errorf("object is not Boolean. got=%T (%+v)", obj, obj)
		return false
	}

	if result.Value != expected {
		t.Errorf("object has wrong value. got=%t, want=%t", result.Value, expected)
		return false
	}

	return true
}

func testFunctionObject(t *testing.T, obj object.Object) bool {
	_, ok := obj.(*object.Function)
	return ok
}

func testStringObject(t *testing.T, obj object.Object, expected string) bool {
	result, ok := obj.(*object.String)
	if !ok {
		t.Errorf("object is not String. got=%T (%+v)", obj, obj)
		return false
	}

	if result.Value != expected {
		t.Errorf("expected Value to be %s. got=%s", expected, result.Value)
		return false
	}

	return true
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) bool {
	result, ok := obj.(*object.Integer)
	if !ok {
		t.Errorf("object is not Integer. got=%T (%+v)", obj, obj)
		return false
	}

	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d", result.Value, expected)
		return false
	}

	return true
}

func TestBangOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
		// {"!0", true},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestIfElseExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"if (true) { 10 }", 10},
		{"if (false) { 10 }", nil},
		{"if (1) { 10 }", 10},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 > 2) { 10 }", nil},
		{"if (1 > 2) { 10 } else { 20 }", 20},
		{"if (1 < 2) { 10 } else { 20 }", 10},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func testNullObject(t *testing.T, obj object.Object) bool {
	if obj != NULL {
		t.Errorf("expected NULL. got=%T (%+v)", obj, obj)
		return false
	}
	return true
}

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"return 10;", 10},
		{"return 50;", 50},
		// {"return true;", true},
		// {"return 10 > 5; 1;", true},
		// {"return 2 < 1;", false},
		// {"return 2 < 1; true;", false},
		{"return 2 * 6; true;", 12},
		{
			`
if (10 > 1) {
  if (10 > 1) {
    return 10;
  }

  return 1;
}
`,
			10,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		ret, ok := evaluated.(*object.ReturnValue)
		if !ok {
			t.Fatalf("expected evaludated to be *object.ReturnValue. got=%T", evaluated)
		}
		testIntegerObject(t, ret.Value, tt.expected)
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{
			"5 + true;",
			"type mismatch: INTEGER + BOOLEAN",
		},
		{
			"5 + true; 5;",
			"type mismatch: INTEGER + BOOLEAN",
		},
		{
			"-true",
			"unknown operator: -BOOLEAN",
		},
		{
			"true + false;",
			"unknown operator: BOOLEAN + BOOLEAN",
		},
		{
			"5; true + false; 5",
			"unknown operator: BOOLEAN + BOOLEAN",
		},
		{
			"if (10 > 1) { true + false; }",
			"unknown operator: BOOLEAN + BOOLEAN",
		},
		{
			`
        if (10 > 1) {
          if (10 > 1) {
            return true + false;
          }

          return 1;
        }
        `,
			"unknown operator: BOOLEAN + BOOLEAN",
		},
		{
			"foobar",
			"identifier not found: foobar",
		},
		{
			`"foobar" * "world"`,
			"unknown operator: STRING * STRING",
		},
		{
			"{2:2}[fn(){}]",
			"hash index cannot be FUNCTION",
		},
	}

	for idx, tt := range tests {
		evaluated := testEval(tt.input)

		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Errorf("[%d] no error object returned. got=%T(%+v)",
				idx, evaluated, evaluated)
			continue
		}

		if errObj.Message != tt.expectedMessage {
			t.Errorf("wrong error message. expected=%q, got=%q",
				tt.expectedMessage, errObj.Message)
		}
	}
}

func TestLetStatement(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"let x = 10; x;", 10},
		{"let x = 5; x;", 5},
		{"let a = -1; a;", -1},
		{"let a = 1; let b = 6; b;", 6},
		{"let a = 1; let b = 6; b; a;", 1},
	}

	for _, tt := range tests {
		testIntegerObject(t, testEval(tt.input), tt.expected)
	}
}

// params and body (as strs)
func TestFunctionObject(t *testing.T) {
	input := "fn (a, b) { a + b; };"

	evaluated := testEval(input)
	fn, ok := evaluated.(*object.Function)
	if !ok {
		t.Fatalf("expected evaludated to be *object.Function. got=%T", evaluated)
	}

	if len(fn.Parameters) != 2 {
		t.Fatalf("expected to have 2 params. got=%d", len(fn.Parameters))
	}

	if fn.Parameters[0].String() == "x" {
		t.Fatalf("param[0] is not x. got=%s", fn.Parameters[0].String())
	}

	expectedBody := "(a + b)" // cuz wrapping/grouping stuff
	if fn.Body.String() != expectedBody {
		t.Fatalf("expected body to be %s. got=%s", fn.Body.String(), expectedBody)
	}
}

// applying function
func TestFunctionApplication(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"let a = fn (a, b) { return a + b; }; a(1, 2);", 3},
		{"let a = fn (a, b) { return a * b; }; a(3, 2);", 6},
		{"let a = fn (a) { return a * 10; }; a(9);", 90},
		{"fn (a) { return a * 10; }(9);", 90},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestClosures(t *testing.T) {
	input := `
		let multiply = fn (x) {
			return fn (y) {
				return x * y;
			};
		};

		let multbytwo = multiply(2)
		multbytwo(6);
	`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 12)
}

func TestStringLiteral(t *testing.T) {
	input := `"Hello World!"`
	evaluated := testEval(input)
	str, ok := evaluated.(*object.String)
	if !ok {
		t.Fatalf("expected evaluated to be *object.String. got=%T", evaluated)
	}

	if str.Value != "Hello World!" {
		t.Fatalf("expected evaluated.Value to be \"Hello World!\". got=%s", str.Value)
	}
}

func TestStringConcat(t *testing.T) {
	input := `"Hello" + " " + "World!"`
	evaluated := testEval(input)
	str, ok := evaluated.(*object.String)
	if !ok {
		t.Fatalf("expected evaluated to be *object.String. got=%T (%+v)", evaluated, evaluated)
	}

	if str.Value != "Hello World!" {
		t.Fatalf("expected evaluated.Value to be \"Hello World!\". got=%s", str.Value)
	}
}

func TestStringComparison(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`"foo" == "foo"`, true},
		{`"foo" != "foo"`, false},
		{`"foo" != "bar"`, true},
		{`"foo" == "bar"`, false},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		bl, ok := evaluated.(*object.Boolean)
		if !ok {
			t.Fatalf("expected evaluated to be *object.Boolean. got=%T", evaluated)
		}
		if bl.Value != tt.expected {
			t.Fatalf("expected %t. got=%t", tt.expected, bl.Value)
		}
	}
}

func TestBuiltinFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{`len("")`, 0},
		{`len("four")`, 4},
		{`len("hello world")`, 11},
		{`len(1)`, "argument to `len` is not supported. got=INTEGER"},
		{`len("one", "two")`, "wrong number of arguments. expected=1. got=2"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)

		switch expected := tt.expected.(type) {
		case int:
			testIntegerObject(t, evaluated, int64(expected))
		case string:
			errObj, ok := evaluated.(*object.Error)
			if !ok {
				t.Fatalf("expected evaludated to be an *object.Error. got=%T (%+v)", evaluated, evaluated)
			}

			if errObj.Message != expected {
				t.Fatalf("expected error to be '%s'. got='%s'", tt.expected, errObj.Message)
			}
		}
	}
}

func TestArrayLiterals(t *testing.T) {
	input := "[1 + 2, 6 * 9, \"strings are expensive\", fn () {}]"
	evaluated := testEval(input)

	arr, ok := evaluated.(*object.Array)
	if !ok {
		t.Fatalf("expected *object.Array. got=%T", evaluated)
	}

	testIntegerObject(t, arr.Elements[0], 3)
	testIntegerObject(t, arr.Elements[1], 54)
	testStringObject(t, arr.Elements[2], "strings are expensive")
	testFunctionObject(t, arr.Elements[3])
}

func TestArrayIndexExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{
			"[1, 2, 3][0]",
			1,
		},
		{
			"[1, 2, 3][1]",
			2,
		},
		{
			"[1, 2, 3][2]",
			3,
		},
		{
			"let i = 0; [1][i];",
			1,
		},
		{
			"[1, 2, 3][1 + 1];",
			3,
		},
		{
			"let myArray = [1, 2, 3]; myArray[2];",
			3,
		},
		{
			"let myArray = [1, 2, 3]; myArray[0] + myArray[1] + myArray[2];",
			6,
		},
		{
			"let myArray = [1, 2, 3]; let i = myArray[0]; myArray[i]",
			2,
		},
		{
			"[1, 2, 3][3]",
			nil,
		},
		{
			"[1, 2, 3][-1]",
			nil,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else if !isError(evaluated) {
			t.Fatalf("expected error. got=%T", evaluated)
		}
	}
}

func TestBuiltins(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"len(\"hello\")", "5"},
		{"first([1,2,3])", "1"},
		{"last([1,2,3])", "3"},
		{"rest([1,2,3])", "[2, 3]"},
		{"push([1,2,3], 6)", "[2, 3, 6]"},
	}

	for _, tt := range tests {
		evaludated := testEval(tt.expected)

		if evaludated.Inspect() != tt.expected {
			t.Fatalf("expected %s. got=%s", tt.expected, evaludated.Inspect())
		}
	}
}

func TestStringHashKey(t *testing.T) {
	hello1 := &object.String{Value: "hello"}
	hello2 := &object.String{Value: "hello"}
	diff1 := &object.String{Value: "my name is skrillex"}
	diff2 := &object.String{Value: "my name is skrillex"}

	if hello1.HashKey() != hello2.HashKey() {
		t.Errorf("strings with same content should have same hash keys")
	}

	if diff1.HashKey() != diff2.HashKey() {
		t.Errorf("strings with same content should have same hash keys")
	}

	if hello1.HashKey() == diff1.HashKey() {
		t.Errorf("strings with different content should differ")
	}
}

func TestIntegerHashKey(t *testing.T) {
	hello1 := &object.Integer{Value: 1}
	hello2 := &object.Integer{Value: 1}
	diff1 := &object.Integer{Value: 2}
	diff2 := &object.Integer{Value: 2}

	if hello1.HashKey() != hello2.HashKey() {
		t.Errorf("integers with same value should have same hash keys")
	}

	if diff1.HashKey() != diff2.HashKey() {
		t.Errorf("integers with same value should have same hash keys")
	}

	if hello1.HashKey() == diff1.HashKey() {
		t.Errorf("integers with value content should differ")
	}
}

func TestBooleanHashKey(t *testing.T) {
	hello1 := &object.Boolean{Value: true}
	hello2 := &object.Boolean{Value: true}
	diff1 := &object.Boolean{Value: false}
	diff2 := &object.Boolean{Value: false}

	if hello1.HashKey() != hello2.HashKey() {
		t.Errorf("same booleans should have same hash keys")
	}

	if diff1.HashKey() != diff2.HashKey() {
		t.Errorf("same booleans should have same hash keys")
	}

	if hello1.HashKey() == diff1.HashKey() {
		t.Errorf("different booleans should have different hash keys")
	}
}

func TestHashKeysLog(t *testing.T) {
	t.SkipNow()
	name1 := &object.String{Value: "name"}
	monkey := &object.String{Value: "Monkey"}

	pairs := map[object.HashKey]object.Object{}
	pairs[name1.HashKey()] = monkey

	fmt.Printf("pairs[name1.HashKey()]=%+v\n", pairs[name1.HashKey()])

	name2 := &object.String{Value: "name"}

	fmt.Printf("pairs[name2.HashKey()]=%+v\n", pairs[name2.HashKey()])
	fmt.Printf("(name1 == name2)=%t\n", name1 == name2)
	fmt.Printf("(name1.HashKey() == name2.HashKey())=%t\n", name1.HashKey() == name2.HashKey())

}

func TestHashLiterals(t *testing.T) {
	input := `let two = "two";
	{
		"one": 10 - 9,
		two: 1 + 1,
		"thr" + "ee": 6 / 9,
		4: 4,
		true: 5,
		false: 6
	}
`
	evaluated := testEval(input)
	result, ok := evaluated.(*object.Hash)
	if !ok {
		t.Errorf("evaluated expected *object.Hash. got=%T", evaluated)
		if isError(evaluated) {
			t.Log(evaluated.Inspect())
		}
		t.FailNow()
	}

	expected := map[object.HashKey]int64{
		(&object.String{Value: "one"}).HashKey():   1,
		(&object.String{Value: "two"}).HashKey():   2,
		(&object.String{Value: "three"}).HashKey(): 0,
		(&object.Integer{Value: 4}).HashKey():      4,
		TRUE.HashKey():                             5,
		FALSE.HashKey():                            6,
	}

	for expectedKey, expectedValue := range expected {
		pair, ok := result.Pairs[expectedKey]
		if !ok {
			t.Fail()
		}
		testIntegerObject(t, pair.Value, expectedValue)
	}
}

func TestHashIndexExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{
			`{"foo": 5}["foo"]`,
			5,
		},
		{
			`{"foo": 5}["bar"]`,
			nil,
		},
		{
			`let key = "foo"; {"foo": 5}[key]`,
			5,
		},
		{
			`{}["foo"]`,
			nil,
		},
		{
			`{5: 5}[5]`,
			5,
		},
		{
			`{true: 5}[true]`,
			5,
		},
		{
			`{false: 5}[false]`,
			5,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func TestLoadMacro(t *testing.T) {
	input := `let sc = load("eval/test_data.eye", true); sc;`

	evaluated := testEval(input)
	if isError(evaluated) {
		t.Errorf("load macro failed. got=%s", evaluated.Inspect())
	}

	sc, ok := evaluated.(*object.Scope)
	if !ok {
		t.Fatalf("load macro failed. got=%T", sc)
	}

	num, ok := sc.Outer.Get("num")
	if !ok {
		t.Fatalf("load macro failed. num not found")
	}

	testIntegerObject(t, num, 2)
}

func TestListsLib(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"push([1, 2, 3], 66)", []int64{1, 2, 3, 66}},
		{"rest([1, 2, 3])", []int64{2, 3}},
		{"first([1, 2, 3])", 1},
		{"last([1, 2, 3])", 3},
	}

	for _, tt := range tests {
		expected := tt.expected.([]int64)
		evaluated := testEval(tt.input)

		arr, ok := evaluated.(*object.Array)
		if ok {
			if len(arr.Elements) != len(expected) {
				t.Errorf("expected %d elements. got=%d", len(expected), len(arr.Elements))
			}

			for i, el := range arr.Elements {
				testIntegerObject(t, el, expected[i])
			}
			return
		}

		num, ok := evaluated.(*object.Integer)
		if ok {
			testIntegerObject(t, evaluated, num.Value)
			return
		}

		t.Fatalf("expected *object.Array or *object.Integer. got=%T", evaluated)
	}
}

// Currying vs Partial Application ?
func TestCurrying(t *testing.T) {
	input := `
	let add = fn(a, b) { a + b };
	# detect why nil pointer dereference
	let addtwo = add(2);
	addtwo(3);
	`

	evaluated := testEval(input)
	if isError(evaluated) {
		t.Fatalf("currying failed. got=%s", evaluated.Inspect())
	}
	testIntegerObject(t, evaluated, 5)
}

func TestPipeExpression(t *testing.T) {
	input := `
		let add = fn(a, b) { a + b };
		let square = fn(a) { a * a };
		let addAndSquare = add(2) |> square;
		add
		`
	t.Fatalf(input)
}

func TestScopes(t *testing.T) {
	t.Fatalf("not implemented")
}

func TestExamplesInterpretingSuccessfully(t *testing.T) {
	files, err := os.ReadDir("examples")
	if err != nil {
		t.Fatalf("failed to read examples directory: %s", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.HasSuffix(file.Name(), ".eye") {
			t.Logf("interpreting %s", file.Name())
			input, err := os.ReadFile(fmt.Sprintf("examples/%s", file.Name()))
			if err != nil {
				t.Fatalf("failed to read file: %s", err)
			}

			evaluated := testEval(string(input))
			if isError(evaluated) {
				t.Fatalf("failed to interpret %s: %s", file.Name(), evaluated.Inspect())
			}
		}
	}
}

func TestFunctionShorthandNotation(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"(fn a, b: a + b)(5, 6)", int64(11)},
		{"(fn (a, b): a + b)(5, 6)", int64(11)},
		{"(fn (a, b): { a + b })(5, 6)", int64(11)},
		{"(fn: 5 + 6)()", int64(11)},
		// why this isn't being catched?
		// {"(fn (x, y, z): { x**2 + y**2 + z**2 })(1, 2, 3)", int64(14)},
		{"(fn: { \"Heey!\" })()", "Heey!"},
		{"(fn(a, b) { a*b })(5, 5)", int64(25)}, // regression just in case
	}

	// t.Fatal(tests)
	for _, tt := range tests {
		evaluated := testEval(tt.input)
		if isError(evaluated) {
			t.Fatalf("failed to interpret %s: %s", tt.input, evaluated.Inspect())
		}
		switch objType := tt.expected.(type) {
		case int64:
			testIntegerObject(t, evaluated, objType)
		case string:
			testStringObject(t, evaluated, objType)
		}
	}
}
