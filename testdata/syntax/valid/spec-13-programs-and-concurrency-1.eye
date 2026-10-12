# SPECS.md section 13, example 1.
import fs

pub fn main(args: List(String)) -> Result((), fs.FsError) uses {fs, stdio} {
  for path in args |> drop(1) {
    let lines = fs.lines(path)?
    print("{path}: {lines |> size} lines")
  }
  Ok(())
}
