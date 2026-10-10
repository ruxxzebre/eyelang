type CountError = | Usage | Unreadable(FsError) | BadNumber(ParseError)

fn count(path: String) -> Result(Int, CountError) uses {fs} {
  let text = fs.readText(path)?
  let n = parseInt(text)?
  Ok(n)
}

fn firstWord(text: String) -> Option(String) {
  let [word, ..] = text |> words else { return None }
  Some(word)
}

fn explicit(path: String) -> Result(Int, CountError) uses {fs} {
  let text = fs.readText(path).mapErr(Unreadable)?
  parseInt(text).mapErr(fn e: BadNumber(e))
}

fn checked(xs: List(Int), i: Int) -> Int {
  if i >= size(xs) { unreachable("callers check the length first") }
  xs[i] ?? 0
}

fn averages(total: Int, count: Int) -> Int {
  match nonzero(count) {
    Some(n) => total / n
    None => 0
  }
}
