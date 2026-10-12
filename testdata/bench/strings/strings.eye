# Strings: build one long string by interpolation and `+=`, split it apart
# again, and look at the pieces.

pub fn main(args: List(String)) uses {stdio} {
  let n = match parseInt(args[1] ?? "") {
    Ok(n) => n
    Err(_) => 1_000_000
  }
  var text = ""
  for i in 0..n {
    text += "item{i % 1000}:{i};"
  }
  let parts = text.split(";")
  var chars = 0
  var sevens = 0
  for part in parts {
    chars += part.size()
    if part.startsWith("item7") { sevens += 1 }
  }
  print("{text.size()} characters, {parts.size()} parts, {chars} in parts, {sevens} start with item7")
}
