# A branch that is one assignment needs no braces, like one that is one
# expression.

fn gather(results: List(Result(String, Error))) {
  var found = []
  var problems = 0
  for r in results {
    match r {
      Ok(text) => found += [text]
      Err(_) => problems += 1
    }
  }
  if problems > 3 then found = [] else problems = 0
  var last = ""
  for text in found {
    if text != "" then last = text
  }
}
