package cli

import (
	"eyelang/eval"
	"eyelang/lexer"
	"eyelang/object"
	"eyelang/parser"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EvalString evaluates code passed on the command line in env and prints its result.
// The returned error is non-nil when parsing or evaluation failed.
func EvalString(input string, env *object.Environment) error {
	obj, err := evalSource(input, env)
	if err != nil {
		return err
	}

	if obj != nil {
		fmt.Println(obj.Inspect())
	}

	return nil
}

// EvalFile evaluates a script file in env.
// The returned error is non-nil when the file could not be read, parsed or evaluated.
func EvalFile(path string, env *object.Environment) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	if _, err := evalSource(string(content), env); err != nil {
		return fmt.Errorf("[%s]: %w", filepath.Base(path), err)
	}

	return nil
}

func evalSource(input string, env *object.Environment) (object.Object, error) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		errors := []string{}
		for _, error := range p.Errors() {
			errors = append(errors, error.Message)
		}
		return nil, fmt.Errorf("parsing errors:\n%s", strings.Join(errors, "\n"))
	}

	obj := eval.Eval(program, env)
	if err, ok := obj.(*object.Error); ok {
		return nil, fmt.Errorf("error encountered: %s", err.Inspect())
	}

	return obj, nil
}
