---
title: "Errors: Result only"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Errors: Result only

- Expected failures are values: `Result(T, E)` and domain variants such as `Refused` and
  `Uncertain`. Postfix `?` returns the error to the caller: `let p = Player(id).profile()?`. In a
  function that returns an `Option`, `?` on an `Option` returns `None`.
- **`?` needs somewhere to return to.** On an `Option` it needs a function that returns an
  `Option`, and on a `Result` one that returns a `Result`; anywhere else it is an error that says
  what the function returns. For a `None` there, give a default with `??`, match the `Option`, or
  make the function return an `Option`. The same holds in a job and in a lambda, and a lambda's
  `?` returns from the lambda, not from the function it is written in, so the lambda's own result
  decides, and an error is wrapped into the lambda's error type. `?` needs to know whether its
  value is an `Option` or a `Result`; the rest of the function may say, as a call
  `f(Some(1))` gives `let f = fn x: Some(x? + 1)` its parameter's type. When nothing says, write
  the type where the value is bound: `fn (x: Option(Int)): …`, or `fn g(x: Option(Int)) = …`
  for a private function's parameter.

  ```eye
  fn next(x: Option(Int)) -> Int = x? + 1          # error: next returns Int, not an Option
  fn nextOr(x: Option(Int)) -> Int = (x ?? 0) + 1
  fn doubled(xs: List(Option(Int))) -> List(Option(Int)) = xs |> map(fn x: Some(x? * 2))
  ```
- **A `Result` can't be dropped.** A statement whose value is a `Result` must use it, match it,
  pass its error on with `?`, or discard it with `let _ = …`; dropping it without a word is an
  error that names its type. A command's outcome follows the same rule ([section 17](/reference/language/contracts-talking-to-other-services)).
  - The last expression of a block is used when the block's value is: the body of a function that
    returns a `Result`, or a branch of an `if` or a `match` whose value is bound or returned.
  - The last expression of a function that returns `()`, a loop's body, the branch of an `if`
    without `else`, and the arms of a `match` whose value nothing uses are statements.
  - Other values may be dropped: `()`, which `print`, `log.info` and every function without `->`
    return, and an `Option`, which is a value that may be missing rather than a failure.

  ```eye
  import fs

  fn save(path: String, text: String) uses {fs} {
    let _ = fs.writeText("{path}.bak", text)   # dropped on purpose: a backup is a bonus
    fs.writeText(path, text)                   # error: this drops a Result((), FsError)
  }
  ```
- **Converting errors.** When the caller's error type has exactly one variant whose payload is the
  error's type, `?` wraps the error in it. With none or several, write the conversion:
  `.mapErr(Variant)?` or `.mapErr(fn e: …)?`.

  ```eye
  import fs

  type CountError = | Usage | Unreadable(fs.FsError) | BadNumber(String)

  fn count(path: String) -> Result(Int, CountError) uses {fs} {
    let text = fs.readText(path)?        # an fs.FsError becomes Unreadable(…)
    let n = parseInt(text)?              # parseInt's message becomes BadNumber(…)
    Ok(n)
  }
  ```
- **Programs that only report errors** can use the standard library's `Problem`: `?` turns any
  error with `Show` into a `Problem`, and `.context("reading {path}")` adds a line to it. A
  `Problem` shows its lines outermost first, then the error: `reading eye.toml: not a whole
  number: "x"`.
- There are no exceptions. Library functions are total: `xs[0]` and `first(xs)` return `Option`,
  and `parseInt(s)` returns `Result`.
- **`unreachable("why")`** is the one way for code to stop itself. It ends the program, or the
  job's run, as a fault: `Failed(Unreachable("why"))`. It is for states the code has already ruled
  out, such as an index a loop just checked, never for failures that can happen. The two
  operations whose misuse can only be a bug stop the same way: assigning an element out of range
  ([section 7](/reference/language/mutation)) and shifting by a negative count ([section 5](/reference/language/values-and-types)).
- **Arithmetic is total.** `Int` has arbitrary precision, so it can't overflow. Small values stay
  unboxed at run time. `/` and `%` take a `NonZero(Int)` divisor. Literals other than `0` qualify
  automatically; for any other value, `nonzero(n)` returns `Option(NonZero(Int))`. Other
  arithmetic takes a `NonZero(Int)` as an `Int` and gives an `Int`, which may be 0, so a
  `let k = 4` that divides can also scale: `k * 1 h` and `1 h / k`. `Float` follows IEEE 754,
  with one exception for equality (below), so dividing by zero gives an infinity rather than a
  failure.

  ```eye
  let avg = total / 3
  match nonzero(count) {
    Some(n) => total / n
    None => 0
  }
  ```

  Two kinds of division come up often enough to need no `match`. `xs.average()` is the mean of a
  list of `Int`s or `Float`s as a `Float`, or `None` for an empty list: `[1, 2].average()` is
  `Some(1.5)`. It gives a `Float` for `Int`s too, so its type is the same for every list and a
  mean of `1` and `2` isn't quietly `1`. `a.div(b)` and `a.rem(b)` are `a / b` and `a % b` for
  `Int`s, or `None` when `b` is `0`. They round as `/` and `%` do, down, so `div(-7, 2)` is
  `Some(-4)` and `rem(-7, 2)` is `Some(1)`.

  ```eye
  fn summary(latencies: List(Int), sent: Int, failed: Int) -> String {
    let mean = latencies.average() ?? 0.0
    let failedPerHundred = (failed * 100).div(sent) ?? 0
    "{mean} ms on average, {failedPerHundred} of every 100 failed"
  }
  ```

  For an `Int` mean that rounds as `/` does, write `xs.sum().div(xs.size())`.

  `0.0 / 0.0` gives NaN, "not a number". `Show` writes the infinities and NaN as `+Inf`, `-Inf`
  and `NaN`; any other `Float` with the fewest digits that read back as it, and `.0` when it would
  otherwise look like an `Int`: `2.0`, `0.25`, `6.02e+23`, `-0.0`. JSON has no NaN or
  infinities, so a value that holds one can't be written as JSON ([section 17](/reference/language/contracts-talking-to-other-services)).

  **Equality is the one exception to IEEE 754.** `==` on `Float`s asks whether two values are the
  same, so `NaN == NaN` is true and `0.0 == -0.0` is false, where IEEE 754 says the opposite.
  Every value then equals itself: a NaN is found in a list, a set or a map's keys, two records
  that hold one are equal, and a test can `expect` one. `x == nan` tests whether `x` is NaN.

  ```eye
  test "NaN equals itself" {
    let zero = 0.0
    let nan = zero / zero
    expect(nan == nan && [nan].contains(nan) && !(zero == -zero))
    expect("{nan} {1.0 / zero} {-1.0 / zero}" == "NaN +Inf -Inf")
  }
  ```
- **Runtime faults** (out of memory, over budget, killed, deadline passed, `unreachable`) are not
  errors that code can throw or catch. They end the run, and whoever started it sees `Failed(fault)` as the run's
  outcome.
