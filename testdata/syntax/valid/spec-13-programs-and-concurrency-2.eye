# SPECS.md section 13, example 2.
# The example is a list of statements, wrapped in a function.
fn example() {
  let pages = urls |> parallelMap(fn u: http.get(u), limit: 8)    # results in input order
  let (user, orders) = both(fn { users.get(id) }, fn { orders.list(id) })
  let fastest = race([fn { mirrorA.get(path) }, fn { mirrorB.get(path) }])
}
