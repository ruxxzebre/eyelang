# SPECS.md section 4, example 2.
# The example is a list of statements, wrapped in a function.
fn example() {
  let ready = worlds
    |> filter(fn w: w.phase == Running)      # continues: the line starts with |>
    |> size
  let next = a
  (b)                                        # a new statement, not the call a(b)
}
