# SPECS.md section 10, example 1.
fn next(x: Option(Int)) -> Int = x? + 1          # error: next returns Int, not an Option
fn nextOr(x: Option(Int)) -> Int = (x ?? 0) + 1
fn doubled(xs: List(Option(Int))) -> List(Option(Int)) = xs |> map(fn x: Some(x? * 2))
