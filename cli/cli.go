package cli

import (
	"eyelang/eval"
	"eyelang/lexer"
	"eyelang/object"
	"eyelang/parser"
	"fmt"
	"log"
	"os"
	"strings"
)

func EvalString(input string) *object.Environment {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	errors := []string{}
	for _, error := range p.Errors() {
		errors = append(errors, error.Message)
	}

	if len(errors) > 0 {
		log.Fatalf("parsing errors: %s", strings.Join(errors, "\n"))
	}

	env := object.NewEnvironment()
	expr := eval.Eval(program, env)

	fmt.Println(expr.Inspect())

	return env
}

func EvalFile(filepath string) *object.Environment {
	// file, err := os.Open(filepath)
	content, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatalf("could not read file: %s", err)
	}

	l := lexer.New(string(content))
	p := parser.New(l)
	program := p.ParseProgram()

	errors := []string{}
	for _, error := range p.Errors() {
		errors = append(errors, error.Message)
	}

	if len(errors) > 0 {
		log.Fatalf("parsing errors: %s", strings.Join(errors, "\n"))
	}

	scope := eval.InitReplScope()
	env := scope.Outer
	// expr := eval.Eval(program, env)
	obj := eval.Eval(program, env)
	if err, ok := obj.(*object.Error); ok {
		fmt.Printf(err.Inspect())
	}

	// fmt.Println(expr.Inspect())
	return env
}
