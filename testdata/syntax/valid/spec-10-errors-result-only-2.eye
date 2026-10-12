# SPECS.md section 10, example 2.
import fs

fn save(path: String, text: String) uses {fs} {
  let _ = fs.writeText("{path}.bak", text)   # dropped on purpose: a backup is a bonus
  fs.writeText(path, text)                   # error: this drops a Result((), FsError)
}
