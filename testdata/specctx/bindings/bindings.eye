# What SPECS.md §6's statements assume (context=bindings): the values and
# functions they use.

type Item = { name: String }

let urls = ["https://example.com/a", "https://example.com/b"]

let items: List(Item) = [{ name: "apples" }, { name: "pears" }]

let n = 10

let limit = 5

let retries = 4

fn fetchSize(url: String) -> Int = size(url)

fn sub(a: Int, b: Int) -> Int = a - b

fn fib(n: Int) -> Int = if n < 2 then n else fib(n - 1) + fib(n - 2)
