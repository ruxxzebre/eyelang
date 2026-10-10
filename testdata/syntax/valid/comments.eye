# A file full of comments.

# Before a declaration.
fn f(  # after an open parenthesis
  a: Int,  # after a parameter
  # between parameters
  b: Int,
) -> Int {  # after a brace
  # at the start of a block
  let x = a  # after a statement
    # between continued lines
    |> double  # after a continued line
  # before the last expression
  x + b  # at the end
  # before the closing brace
}
# At the end of the file.
