package eval

import "testing"

func BenchmarkSimple(b *testing.B) {
	input := `
		let fib = fn(n) {
			let iter = fn (a, iters) {
				if (iters == 0) { return a }
				return iter(a + 1, iters - 1) + iter(a + 2, iters - 2)
			}
			iter(0, n)
		}
	`
}
