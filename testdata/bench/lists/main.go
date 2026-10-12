// Lists: map, filter and sum over a slice of whole numbers, each step making
// a new slice as Eyelang's list functions do, then sort pseudo-random ones.
package main

import (
	"fmt"
	"os"
	"slices"
	"strconv"
)

func main() {
	n := 1_000_000
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}
	xs := make([]int, n)
	for i := range xs {
		xs[i] = i
	}
	squares := make([]int, len(xs))
	for i, x := range xs {
		squares[i] = x * x % 1000
	}
	var even []int
	for _, x := range squares {
		if x%2 == 0 {
			even = append(even, x)
		}
	}
	total := 0
	for _, x := range even {
		total += x
	}
	seed := 42
	var ys []int
	for range n {
		seed = (seed*1103515245 + 12345) % 2147483648
		ys = append(ys, seed%1_000_000)
	}
	slices.Sort(ys)
	fmt.Printf("sum %d; sorted %d: %d … %d … %d\n", total, n, ys[0], ys[n/2], ys[n-1])
}
