# SPECS.md section 10, example 3.
import fs

type CountError = | Usage | Unreadable(fs.FsError) | BadNumber(String)

fn count(path: String) -> Result(Int, CountError) uses {fs} {
  let text = fs.readText(path)?        # an fs.FsError becomes Unreadable(…)
  let n = parseInt(text)?              # parseInt's message becomes BadNumber(…)
  Ok(n)
}
