let fib = fn(n) {
	let iter = fn (a, b, iters) {
        if (iters == n) { return a }
		return iter(b, a + b, iters + 1)
	}
	return iter(0, 1, 0)
}

puts(map([]))
puts(fib(5))
