let map = fn (arr, f) {
    let iter = fn (arr, acum) {
        if (len(arr) == 0) {
            return acum
        }
        iter(rest(arr), push(acum, f(first(arr))))
    }

    iter(arr, [])
}

let reduce = fn (arr, f, init) {
    let iter = fn (arr, acum) {
        if (len(arr) == 0) {
            return acum
        }
        iter(rest(arr), f(acum, first(arr)))
    }
    iter(arr, init)
}

let filter = fn (arr, f) {
    let iter = fn (arr, acum) {
        if (len(arr) == 0) {
            return acum
        }
        let first = first(arr)
        if (f(first)) {
            iter(rest(arr), push(acum, first))
        } else {
            iter(rest(arr), acum)
        }
    }

    iter(arr, [])
}

let sum = fn (arr) {
    return reduce(arr, fn (acc, cur) {
        acc + cur
    }, 0)
}

puts(
    sum(
        map(
            filter(
                [1, 2, 3, 4],
                fn (x) { x % 2 == 0; }),
                fn (x) { x * 2 }
            )
    )
)
