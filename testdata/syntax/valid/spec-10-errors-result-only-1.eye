# SPECS.md section 10, example 1.
type CountError = | Usage | Unreadable(FsError) | BadNumber(ParseError)

fn count(path: String) -> Result(Int, CountError) uses {fs} {
  let text = fs.readText(path)?        # an FsError becomes Unreadable(…)
  let n = parseInt(text)?              # a ParseError becomes BadNumber(…)
  Ok(n)
}
