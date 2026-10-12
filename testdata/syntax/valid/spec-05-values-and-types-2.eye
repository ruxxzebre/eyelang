# SPECS.md section 5, example 2.
fn isComment(src: Bytes) -> Bool = match src {
  ['-', '-', ..] => true
  _ => false
}
