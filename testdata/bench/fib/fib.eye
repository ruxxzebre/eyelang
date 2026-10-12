# Calls and integers: the naive Fibonacci function, fib(n).

fn fib(n: Int) -> Int = if n < 2 then n else fib(n - 1) + fib(n - 2)

pub fn main(args: List(String)) uses {stdio} {
  let n = match parseInt(args[1] ?? "") {
    Ok(n) => n
    Err(_) => 37
  }
  print("fib({n}) = {fib(n)}")
}
