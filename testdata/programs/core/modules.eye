import tally

test "a type's functions are called through its module" {
  let t = tally.Tally.new().add().add()
  expect(t.count() == 2)
  expect(tally.Tally.of(5).add().count() == 6)
}
