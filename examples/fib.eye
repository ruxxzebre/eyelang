let fib = fn(n) {
	let iter = fn (a, b, iters) {
        if (iters == n) { return a }
		return iter(b, a + b, iters + 1)
	}
	return iter(0, 1, 0)
}

let fibr = fn(a) {
    if (a < 2) { a } else {
        fibr(a - 1) + fibr(a - 2)
    }
}

puts(fibr(5))
