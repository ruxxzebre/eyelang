# Tutorial 1: values and functions. A small shopping list, priced in cents.

# region functions
fn double(n: Int) -> Int = n * 2

fn price(cents: Int) -> String {
  let euros = cents / 100
  let rest = cents % 100
  if rest < 10 then "€{euros}.0{rest}" else "€{euros}.{rest}"
}
# endregion functions

# region test-functions
test "functions" {
  expect(double(21) == 42)
  expect(price(250) == "€2.50")
  expect(price(1205) == "€12.05")
}
# endregion test-functions

# region lets
test "let names a value" {
  let apples = 3
  let each = 40
  let total = apples * each
  expect(total == 120)
  expect("{apples} apples cost {price(total)}" == "3 apples cost €1.20")
}
# endregion lets

# region lists
test "lists" {
  let basket = [250, 120, 80]
  expect(basket[0] == Some(250))
  expect(basket[5] == None)
  expect(basket + [99] == [250, 120, 80, 99])
  expect(basket |> sort == [80, 120, 250])
  expect(basket == [250, 120, 80])
  expect(basket |> size == 3)
}
# endregion lists

# region pipes
fn total(basket: List(Int)) -> Int = basket |> sum

fn bargains(basket: List(Int), under: Int) -> List(String) = basket
  |> filter(fn cents: cents < under)
  |> sort
  |> map(price)
# endregion pipes

# region test-pipes
test "pipes pass the value on" {
  let basket = [250, 120, 80]
  expect(total(basket) == 450)
  expect(bargains(basket, under: 200) == ["€0.80", "€1.20"])
  expect(basket |> map(double) == [500, 240, 160])
}
# endregion test-pipes

# region closures
fn discount(percent: Int) = fn cents: cents - cents * percent / 100

test "closures keep what they capture" {
  let tenOff = discount(10)
  expect(tenOff(250) == 225)
  expect([250, 120] |> map(tenOff) == [225, 108])
}
# endregion closures

# region var-for
fn countOver(basket: List(Int), limit: Int) -> Int {
  var count = 0
  for cents in basket {
    if cents > limit then count += 1
  }
  count
}

test "var and for" {
  expect(countOver([250, 120, 80], limit: 100) == 2)
  var sum = 0
  for i in 1..=4 { sum += i }
  expect(sum == 10)
}
# endregion var-for
