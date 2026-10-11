# Writes the digest's text. Pure functions, tested where they are declared.

# region headline
## The first line of a digest: the day, and how many items it holds.
pub fn headline(day: Date, items: Int) -> String {
  let what = if items == 1 { "1 item" } else { "{items} items" }
  "{day.format("%A %-d %B")}: {what}"
}

test "a headline names the day and counts the items" {
  expect(headline(2026-10-12, 3) == "Monday 12 October: 3 items")
  expect(headline(2026-10-13, 1) == "Tuesday 13 October: 1 item")
}
# endregion headline
