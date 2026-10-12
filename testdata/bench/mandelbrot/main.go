// Floats: which points of an n×n grid over [-2, 1]×[-1.5, 1.5] stay in the
// Mandelbrot set for 100 iterations, and how many iterations it took in all.
package main

import (
	"fmt"
	"os"
	"strconv"
)

func escape(cr, ci float64, limit int) int {
	zr, zi := 0.0, 0.0
	i := 0
	for i < limit && zr*zr+zi*zi <= 4.0 {
		t := zr*zr - zi*zi + cr
		zi = 2.0*zr*zi + ci
		zr = t
		i++
	}
	return i
}

func main() {
	n := 1600
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}
	const limit = 100
	size := float64(n)
	inside, steps := 0, 0
	for y := 0; y < n; y++ {
		ci := float64(y)*3.0/size - 1.5
		for x := 0; x < n; x++ {
			cr := float64(x)*3.0/size - 2.0
			i := escape(cr, ci, limit)
			steps += i
			if i == limit {
				inside++
			}
		}
	}
	fmt.Printf("%dx%d: %d inside, %d iterations\n", n, n, inside, steps)
}
