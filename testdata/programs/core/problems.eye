# Problem: an error with what was being done, for programs that only report
# errors (SPECS.md §10).

fn parsed(s: String) -> Result(Int, ParseError) =
  if s == "seven" then Ok(7) else Err(NotANumber(s))

fn total(a: String, b: String) -> Result(Int, Problem) {
  let x = parseInt(a)?                       # a String error
  let y = parsed(b).context("reading {b}")?  # a ParseError, with a line
  Ok(x + y)
}

fn report(a: String, b: String) -> Result(Int, Problem) =
  total(a, b).context("adding {a} and {b}")

test "? turns any error with Show into a Problem" {
  expect(total("1", "seven") == Ok(8))
  expect(total("x", "seven") == Err(Problem(context: [], error: "not a whole number: \"x\"")))
}

test "context adds a line, outermost first" {
  let Err(p) = report("1", "eight") else {
    expect(false)
    return
  }
  expect("{p}" == "adding 1 and eight: reading eight: NotANumber(\"eight\")")
  expect(report("1", "seven") == Ok(8))
}
