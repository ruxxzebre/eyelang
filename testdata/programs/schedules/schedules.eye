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

@on(cron("15 9 * * mon-fri", in: "America/New_York"))
job standup(day: Date) {
  Fired("standup").add!(day, now!())
}

@on(cron("0,30 12 1,15 * *", in: "Europe/Dublin"))
job payday(day: Date) {
  Fired("payday").add!(day, now!())
}

# The policies for daylight-saving changes, chosen.
@on(daily(01:30, in: "Europe/Dublin", onGap: Skip, onOverlap: Second))
job late(day: Date) {
  Fired("late").add!(day, now!())
}

@on(cron("0 * * * *", in: "Europe/Dublin", onOverlap: Both))
job hourly(day: Date) {
  Fired("hourly").add!(day, now!())
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

test "a cron line fires on its zone's clock" {
  with clock(2026-10-09 00:00 in "UTC")  # a Friday
  advance(4 days)
  expect(Fired("standup").days == [2026-10-09, 2026-10-12])
  # 09:15 in New York, on summer time
  expect(Fired("standup").times == [2026-10-09 13:15 in "UTC", 2026-10-12 13:15 in "UTC"])
}

test "a cron line's lists pick its days and times" {
  with clock(2026-10-14 00:00 in "UTC")
  advance(2 days)
  expect(Fired("payday").days == [2026-10-15, 2026-10-15])
  expect(Fired("payday").times == [2026-10-15 11:00 in "UTC", 2026-10-15 11:30 in "UTC"])
}

test "a cron line keeps to its zone's clock after a change" {
  with clock(2026-10-31 00:00 in "UTC")  # after Dublin's change back to winter time
  advance(2 days)
  expect(Fired("payday").times == [2026-11-01 12:00 in "UTC", 2026-11-01 12:30 in "UTC"])
}

test "onGap: Skip doesn't fire a time the clock skips" {
  with clock(2027-03-27 12:00 in "UTC")  # Dublin skips 01:00 to 02:00 on 2027-03-28
  advance(2 days)
  expect(Fired("late").days == [2027-03-29])
  expect(Fired("early").days == [2027-03-28, 2027-03-29])  # Shift, the default
}

test "onOverlap: Second fires a time the clock repeats the second time" {
  with clock(2026-10-24 12:00 in "UTC")  # Dublin repeats 01:00 to 02:00 on 2026-10-25
  advance(1 day)
  expect(Fired("late").times == [2026-10-25 01:30 in "UTC"])
  expect(Fired("early").times == [2026-10-25 00:30 in "UTC"])  # First, the default
}

test "onOverlap: Both fires a time the clock repeats twice, an hour apart" {
  with clock(2026-10-24 23:30 in "UTC")  # 00:30 in Dublin, on summer time
  advance(3 h)
  # 01:00 on summer time, 01:00 again on winter time, then 02:00
  expect(Fired("hourly").times == [2026-10-25 00:00 in "UTC", 2026-10-25 01:00 in "UTC", 2026-10-25 02:00 in "UTC"])
}

test "a time the clock skips, shifted onto another firing, fires once" {
  with clock(2027-03-27 23:30 in "UTC")  # 23:30 in Dublin, on winter time
  advance(3 h)
  # 01:00 is shifted to 02:00, which fires anyway
  expect(Fired("hourly").times == [2027-03-28 00:00 in "UTC", 2027-03-28 01:00 in "UTC", 2027-03-28 02:00 in "UTC"])
}

test "every fires on multiples of its interval" {
  with clock(2026-10-12 09:07 in "UTC")
  advance(30 min)
  expect(Fired("tick").times == [2026-10-12 09:15 in "UTC", 2026-10-12 09:30 in "UTC"])
}
