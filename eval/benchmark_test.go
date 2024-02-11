package eval

// TODO!!!:
// do same benches for node and python
// how many allocs in nodejs
// how many allocs in python

import (
	// "fmt"
	"eyelang/object"
	"fmt"
	// "fmt"
	"testing"
)

var fibArg = int64(2)

func BenchmarkSimple(b *testing.B) {
	input := `
		let fibo = fn(a) {
			if (a < 2) { return a }
			return fibo(a - 1) + fibo(a - 2)
		}

		let fib = fibo
	`
	// # gettin buffer overflow with this one (handle)
	// # if (a == 0) { return a }
	// Eval(program, env)
	getInt := func(n int64) *object.Integer {
		return &object.Integer{Value: n}
	}
	getArgs := func(args ...object.Object) []object.Object {
		return args
	}

	evaluated, env := testEval2(input)
	fibo, ok := evaluated.(*object.Function)
	if !ok {
		b.Fatalf("evaluation should result in *object.Function. got=%T", evaluated)
	}

	fib := func(n int64) object.Object {
		return ApplyFunction(fibo, getArgs(getInt(n)), env)
	}

	// num := int64(10)
	for i := 0; i < b.N; i++ {
		// fmt.Printf("fib(%d)\n", num)
		err := fib(fibArg)
		if isError(err) {
			b.Fatalf("encountered an error %s", err.Inspect())
			b.FailNow()
		}
		// recInput := fmt.Sprintf("fib(%d)", i)
		// hugeFib, _ := testEval3(recInput, env)
		// testEval3(recInput, env)
		// b.Log(hugeFib)
	}
}

func BenchmarkSimpleFullProcess(b *testing.B) {
	input := `
		let fib = fn(a) {
			if (a < 2) { return a }
			return fib(a - 1) + fib(a - 2)
		}
	`
	// # gettin buffer overflow with this one (handle)
	// # if (a == 0) { return a }
	_, env := testEval2(input)
	// num := 10
	fibinp := fmt.Sprintf("fib(%d)", fibArg)
	for i := 0; i < b.N; i++ {
		testEval3(fibinp, env)
	}
}

func BenchmarkIterFullProcess(b *testing.B) {
	input := `
		let fibf = fn(n) {
			let iter = fn (a, b, iters) {
	        if (iters == n) { return a }
				return iter(b, a + b, iters + 1)
			}
			return iter(0, 1, 0)
		}
	`
	// # gettin buffer overflow with this one (handle)
	// # if (a == 0) { return a }
	_, env := testEval2(input)
	// num := 10
	fibinp := fmt.Sprintf("fibf(%d)", fibArg)
	for i := 0; i < b.N; i++ {
		testEval3(fibinp, env)
	}
}

func BenchmarkIter(b *testing.B) {
	input := `
		let fibf = fn(n) {
			let iter = fn (a, b, iters) {
	        if (iters == n) { return a }
				return iter(b, a + b, iters + 1)
			}
			return iter(0, 1, 0)
		}
	`
	// # gettin buffer overflow with this one (handle)
	// # if (a == 0) { return a }
	// Eval(program, env)
	getInt := func(n int64) *object.Integer {
		return &object.Integer{Value: n}
	}
	getArgs := func(args ...object.Object) []object.Object {
		return args
	}

	evaluated, env := testEval2(input)
	fibo, ok := evaluated.(*object.Function)
	if !ok {
		b.Fatalf("evaluation should result in *object.Function. got=%T", evaluated)
	}
	// b.Logf("fn: %s %s", evaluated.Type(), evaluated.Inspect())

	args := getArgs(getInt(fibArg))

	// num := int64(10)
	for i := 0; i < b.N; i++ {
		fib := func(n int64) object.Object {
			return ApplyFunction(fibo, args, env)
		}
		fib(fibArg)
		// ApplyFunction(fibo, args, env)
	}
}

func BenchmarkIterMemo(b *testing.B) {
	input := `
		let memo = fn(f) {
			let cache = {};
			return fn(x) {
				if (has(cache, x)) { return cache[x] }
				update!(cache, x, f(x))
				return cache[x]
			}
		}

		let fibo = fn(a) {
			if (a < 2) { return a }
			return fibo(a - 1) + fibo(a - 2)
		}

		let fibf = fn(n) {
			let iter = fn (a, b, iters) {
	        if (iters == n) { return a }
				return iter(b, a + b, iters + 1)
			}
			return iter(0, 1, 0)
		}

		#references the same fn
		#let fib = memo(fibo)
		let fib = memo(fibo)
	`
	// # gettin buffer overflow with this one (handle)
	// # if (a == 0) { return a }
	// Eval(program, env)
	getInt := func(n int64) *object.Integer {
		return &object.Integer{Value: n}
	}
	getArgs := func(args ...object.Object) []object.Object {
		return args
	}

	evaluated, env := testEval2(input)
	fibo, ok := evaluated.(*object.Function)
	if !ok {
		b.Fatalf("evaluation should result in *object.Function. got=%T", evaluated)
	}
	// b.Logf("fn: %s %s", evaluated.Type(), evaluated.Inspect())

	args := getArgs(getInt(fibArg))

	// num := int64(10)
	for i := 0; i < b.N; i++ {
		fib := func(n int64) object.Object {
			return ApplyFunction(fibo, args, env)
		}
		err := fib(fibArg)
		if isError(err) {
			b.Fatalf("got error %s", err.Inspect())
		}
		// ApplyFunction(fibo, args, env)
	}
}
