# SPECS.md section 5, example 6.
import regex

## Dates written 2026-10-12, as 12/10/2026.
fn irishDates(text: String) -> Result(String, regex.RegexError) {
  let date = regex.parse("(\\d\{4})-(\\d\{2})-(\\d\{2})")?
  Ok(date.replace(text, "$3/$2/$1"))
}
