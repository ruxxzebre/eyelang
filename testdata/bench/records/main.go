// Records: a particle bouncing in a box, its struct replaced on every step.
package main

import (
	"fmt"
	"os"
	"strconv"
)

type particle struct{ x, y, vx, vy, bounces int }

func main() {
	n := 30_000_000
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}
	const side = 1000
	p := particle{x: 0, y: 0, vx: 3, vy: 7}
	for range n {
		p = particle{x: p.x + p.vx, y: p.y + p.vy, vx: p.vx, vy: p.vy, bounces: p.bounces}
		if p.x < 0 || p.x > side {
			p.vx, p.bounces = -p.vx, p.bounces+1
		}
		if p.y < 0 || p.y > side {
			p.vy, p.bounces = -p.vy, p.bounces+1
		}
	}
	fmt.Printf("after %d steps: at (%d, %d), %d bounces\n", n, p.x, p.y, p.bounces)
}
