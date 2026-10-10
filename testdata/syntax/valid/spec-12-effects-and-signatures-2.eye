# SPECS.md section 12, example 2.
pub fn sizes(paths: List(String)) -> List(Int) uses {fs} =
  paths |> map(fn p: fs.readText(p) |> size)    # fs comes from the lambda, not from map
