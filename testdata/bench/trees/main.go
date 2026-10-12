// Allocation: the benchmarks game's binary trees. Many short-lived trees of
// depth 4 to n are built and walked while one long-lived tree stays.
package main

import (
	"fmt"
	"os"
	"strconv"
)

type node struct{ left, right *node }

func build(depth int) *node {
	if depth == 0 {
		return &node{}
	}
	return &node{build(depth - 1), build(depth - 1)}
}

func (n *node) check() int {
	if n == nil {
		return 0
	}
	return 1 + n.left.check() + n.right.check()
}

func main() {
	n := 16
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}
	const minDepth = 4
	maxDepth := max(n, minDepth+2)
	stretch := maxDepth + 1
	fmt.Printf("stretch tree of depth %d\t check: %d\n", stretch, build(stretch).check())
	longLived := build(maxDepth)
	for depth := minDepth; depth <= maxDepth; depth += 2 {
		iterations := 1 << (maxDepth - depth + minDepth)
		sum := 0
		for range iterations {
			sum += build(depth).check()
		}
		fmt.Printf("%d\t trees of depth %d\t check: %d\n", iterations, depth, sum)
	}
	fmt.Printf("long lived tree of depth %d\t check: %d\n", maxDepth, longLived.check())
}
