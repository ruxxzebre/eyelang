# SPECS.md section 15, example 6.
job brew(cups: Int) -> Int {
  sleep!(4 min)
  cups
}

test "a brew is ready after four minutes" {
  let run = brew.start(2)
  expect(run.outcome() == None)
  advance(5 min)
  expect(run.outcome() == Some(Completed(2)))
}
