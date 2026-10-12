# SPECS.md section 12, example 2.
import fs

pub fn missing(paths: List(String)) -> List(String) uses {fs} =
  paths |> filter(fn p: !fs.exists(p))    # fs comes from the lambda, not from filter
