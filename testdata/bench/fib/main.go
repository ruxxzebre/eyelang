// Calls and integers: the naive Fibonacci function, fib(n).
package main

import (
	"fmt"
	"os"
	"strconv"
)

func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func main() {
	n := 37
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}
	fmt.Printf("fib(%d) = %d\n", n, fib(n))
}
