let fibf = fn(n) {
	let iter = fn (a, b, iters) {
        if (iters == n) { return a }
		return iter(b, a + b, iters + 1)
	}
	return iter(0, 1, 0)
}

let fib = fn(a) {
    if (a < 2) { a } else {
        fib(a - 1) + fib(a - 2)
    }
}

puts(fib(10))
