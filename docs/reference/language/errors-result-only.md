---
title: "Errors: Result only"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Errors: Result only

- Expected failures are values: `Result(T, E)` and domain variants such as `Refused` and
  `Uncertain`. Postfix `?` returns the error to the caller: `let p = Player(id).profile()?`. In a
  function that returns an `Option`, `?` on an `Option` returns `None`.
- **Converting errors.** When the caller's error type has exactly one variant whose payload is the
  error's type, `?` wraps the error in it. With none or several, write the conversion:
  `.mapErr(Variant)?` or `.mapErr(fn e: …)?`.

  ```eye
  type CountError = | Usage | Unreadable(FsError) | BadNumber(ParseError)

  fn count(path: String) -> Result(Int, CountError) uses {fs} {
    let text = fs.readText(path)?        # an FsError becomes Unreadable(…)
    let n = parseInt(text)?              # a ParseError becomes BadNumber(…)
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
  out, such as an index a loop just checked, never for failures that can happen.
- **Arithmetic is total.** `Int` has arbitrary precision, so it can't overflow. Small values stay
  unboxed at run time. `/` and `%` take a `NonZero(Int)` divisor. Literals other than `0` qualify
  automatically; for any other value, `nonzero(n)` returns `Option(NonZero(Int))`. `Float` follows
  IEEE 754, so dividing by zero gives an infinity rather than a failure.

  ```eye
  let avg = total / 3
  match nonzero(count) {
    Some(n) => total / n
    None => 0
  }
  ```
- **Runtime faults** (out of memory, over budget, killed, deadline passed, `unreachable`) are not
  errors that code can throw or catch. They end the run, and whoever started it sees `Failed(fault)` as the run's
  outcome.
