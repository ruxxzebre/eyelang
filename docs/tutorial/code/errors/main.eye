# Tutorial 3: errors. Reading an order and taking it from the stock.

# region parse-error
type ParseError =
  | Empty
  | NotANumber(String)
  | TooMany(Int)
# endregion parse-error

# region parse
fn parseQuantity(text: String) -> Result(Int, ParseError) {
  let t = text |> trim
  if t == "" { return Err(Empty) }
  let n = parseInt(t).mapErr(fn _: NotANumber(t))?
  if n > 99 then return Err(TooMany(n))
  Ok(n)
}
# endregion parse

# region test-parse
test "a Result is Ok or Err" {
  expect(parseQuantity(" 3 ") == Ok(3))
  expect(parseQuantity("") == Err(Empty))
  expect(parseQuantity("three") == Err(NotANumber("three")))
  expect(parseQuantity("500") == Err(TooMany(500)))
}
# endregion test-parse

# region match-result
fn message(text: String) -> String = match parseQuantity(text) {
  Ok(n) => "{n} it is"
  Err(Empty) => "how many?"
  Err(NotANumber(t)) => "'{t}' is not a number"
  Err(TooMany(n)) => "{n} is more than we sell at once"
}
# endregion match-result

# region test-match-result
test "match handles each error" {
  expect(message("2") == "2 it is")
  expect(message("") == "how many?")
  expect(message("lots") == "'lots' is not a number")
}
# endregion test-match-result

# region default
test "?? gives a default for a missing value" {
  let stock = { "apples": 5 }
  let apples = stock["apples"] ?? 0
  let pears = stock["pears"] ?? 0
  expect(apples == 5)
  expect(pears == 0)
}
# endregion default

# region order-error
type OrderError =
  | BadQuantity(ParseError)
  | OutOfStock(item: String, left: Int)
# endregion order-error

# region take
fn take(mut stock: Map(String, Int), item: String, text: String) -> Result(Int, OrderError) {
  let n = parseQuantity(text)?
  let left = stock[item] ?? 0
  if n > left { return Err(OutOfStock(item:, left:)) }
  stock = stock |> insert(item, left - n)
  Ok(n)
}
# endregion take

# region test-take
test "? passes the error on, and mut shows what may change" {
  var stock = { "apples": 5, "pears": 1 }
  expect(take(mut stock, "apples", "3") == Ok(3))
  expect(stock["apples"] == Some(2))
  expect(take(mut stock, "pears", "2") == Err(OutOfStock(item: "pears", left: 1)))
  expect(take(mut stock, "pears", "two") == Err(BadQuantity(NotANumber("two"))))
  expect(stock["pears"] == Some(1))
}
# endregion test-take

# region option-question
fn firstWord(text: String) -> Option(String) {
  let word = text |> split(" ") |> find(fn w: w != "")?
  Some(word |> upper)
}

test "? works on Option too" {
  expect(firstWord("  hello there") == Some("HELLO"))
  expect(firstWord("   ") == None)
}
# endregion option-question
