# A multi-line string starts with """ and a line break, and ends with """ on a
# line of its own. One or two quotes inside are text.

let prompt = """
  You write a morning briefing for {reader}.
  Say "Nothing new." when a section is empty, and ""quote"" freely.
    - indented lines keep what is beyond the margin
  Escapes still work: \t and \{braces\}.
  """

fn letter(name: String) -> String = """
  Dear {name},

  {body(name |> size)}
  """

let empty = """
  """
