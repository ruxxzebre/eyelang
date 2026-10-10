# Schedules start jobs on the clock of their zone, and a scheduled job that
# takes a Date gets the firing's date there.

entity Fired(name: String) {
  state days: List(Date) = []
  state times: List(Instant) = []

  pub fn add(day: Date, at: Instant) {
    days += [day]
    times += [at]
  }
}

@on(daily(01:30, in: "Europe/Dublin"))
job early(day: Date) {
  Fired("early").add!(day, now!())
}

@on(weekdays(07:40, in: "Europe/Dublin"))
job work(day: Date) {
  Fired("work").add!(day, now!())
}

@on(every(15 min))
job tick() {
  Fired("tick").add!(2026-01-01, now!())
}

test "weekdays skip Saturday and Sunday" {
  with clock(2026-10-09 07:00 in "Europe/Dublin")  # a Friday
  advance(4 days)
  expect(Fired("work").days == [2026-10-09, 2026-10-12])
}

test "a daily time fires on the zone's clock, with the zone's date" {
  with clock(2026-10-12 00:00 in "UTC")
  advance(1 day)
  expect(Fired("early").days == [2026-10-12])
  expect(Fired("early").times == [2026-10-12 00:30 in "UTC"])  # 01:30 in Dublin, on summer time
}

test "a time the clock skips fires as far after the change" {
  with clock(2027-03-27 12:00 in "UTC")  # Dublin skips 01:00 to 02:00 on 2027-03-28
  advance(2 days)
  expect(Fired("early").times == [2027-03-28 01:30 in "UTC", 2027-03-29 00:30 in "UTC"])
}

test "a time the clock repeats fires the first time" {
  with clock(2026-10-24 12:00 in "UTC")  # Dublin repeats 01:00 to 02:00 on 2026-10-25
  advance(1 day)
  expect(Fired("early").times == [2026-10-25 00:30 in "UTC"])
}

test "every fires on multiples of its interval" {
  with clock(2026-10-12 09:07 in "UTC")
  advance(30 min)
  expect(Fired("tick").times == [2026-10-12 09:15 in "UTC", 2026-10-12 09:30 in "UTC"])
}
