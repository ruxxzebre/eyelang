# The zone of a time may be a name for a Zone value.

pub let dublin: Zone = "Europe/Dublin"

@on(daily(07:40, in: dublin))
job morning(day: Date) {
  sleepUntil!(08:00 in dublin)
}

test "times in a named zone" {
  with clock(2026-10-12 07:39 in dublin)
  let noon = 2026-10-12 12:00 in dublin
  expect(noon > now())
}
