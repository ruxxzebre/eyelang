# `?` wraps an error into the caller's error type when one of its variants
# holds it, whatever the error's type (SPECS.md §8).

type Place = { line: Int, column: Int }

type Reason = String

type LoadError =
  | Unreadable(String)
  | Malformed(Place)
  | Code(Int)

type NoteError = | Note(Reason)

fn readText(path: String) -> Result(String, String) =
  if path == "" then Err("no path") else Ok("text of {path}")

fn decode(text: String) -> Result(Int, Place) =
  if text == "text of bad" then Err({ line: 3, column: 7 }) else Ok(text |> size)

fn status(n: Int) -> Result(Int, Int) = if n > 20 then Err(n) else Ok(n)

fn load(path: String) -> Result(Int, LoadError) {
  let text = readText(path)?
  let n = decode(text)?
  Ok(status(n)?)
}

fn noted(path: String) -> Result(String, NoteError) {
  let text = readText(path)?
  Ok(text)
}

test "? wraps a String, a record and an Int error in the variant that holds it" {
  expect(load("ab") == Ok(10))
  expect(load("") == Err(Unreadable("no path")))
  expect(load("bad") == Err(Malformed({ line: 3, column: 7 })))
  expect(load("a longer name") == Err(Code(21)))
  expect(noted("") == Err(Note("no path")))
}
