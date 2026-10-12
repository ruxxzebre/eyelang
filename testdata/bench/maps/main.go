// Maps: count n words drawn from a vocabulary of 5,000, then find the most
// common count and how many words reached it.
package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	n := 5_000_000
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}
	const vocabulary = 5000
	words := make([]string, 0, vocabulary)
	for i := range vocabulary {
		words = append(words, "w"+strconv.Itoa(i*7919%100_003))
	}
	counts := map[string]int{}
	seed := 7
	for range n {
		seed = (seed*1103515245 + 12345) % 2147483648
		counts[words[seed/16%vocabulary]]++
	}
	top, atTop := 0, 0
	for _, count := range counts {
		if count > top {
			top, atTop = count, 1
		} else if count == top {
			atTop++
		}
	}
	fmt.Printf("%d words; the most common seen %d times, %d of them\n", len(counts), top, atTop)
}
