let five = 5;
let ten = 10;

let add = fn (a, b) {
    a + b;
}

let result = add(five, ten)
result

let name = "Киця";
let sticker = "❤️";

puts("I love " + name + sticker + "!");

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

let mapping = map([1, 2], fn (a) { a * 2 });
puts(mapping);

let reducing = reduce([4, 2, 0], fn (acc, cur) {
    acc + cur
}, 0);

puts(reducing);

let reducin = reduce([4, 2, 0], fn (acc, cur) {
    acc * cur;
}, 1);
puts(reducin);

let sum = fn (arr) {
    return reduce(arr, fn (acc, cur) {
        acc + cur
    }, 0)
}

puts(sum([1,2,3,4,5]));

puts(filter(
    [-1, 2, 4, 10, 0],
    fn (a) { a > 0 }
))
