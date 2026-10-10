# `_` names a parameter that is ignored.

fn f(xs: List(Int)) -> List(String) {
  let constant = fn _: "x"
  let second = fn (_, b): b
  xs |> map(fn _: constant(0))
}
