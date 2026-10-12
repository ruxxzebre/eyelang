# The methods of time values (SPECS.md §20), which native code must give as
# the interpreter does (interp/times.go): an instant's seconds, date and time
# of day in a zone, the next time a clock shows, a date at a time, dates and
# times written out, and ranges of times of day that may wrap midnight.

let dublin = "Europe/Dublin"
let york = "America/New_York"

## The digest's way of writing when an item was published.
fn irishTime(at: Instant) -> String = "{at.date(in: dublin).format("%d %B")} {at.time(in: dublin).format("%H:%M")}"

## Notify's quiet hours: when a message held back may go.
fn quietUntil(quiet: Option(Range(Time)), at: Instant) -> Option(Instant) {
  let Some(hours) = quiet else { return None }
  if !(hours |> contains(at.time(in: dublin))) { return None }
  Some(hours.end.next(in: dublin, after: at))
}

test "an instant's seconds since 1970" {
  expect((1970-01-01 00:00 in "UTC").unix() == 0)
  expect((2026-10-10 12:00 in "UTC").unix() == 1791633600)
  expect((2026-10-10 12:00 in dublin).unix() == 1791630000)
  expect((1969-12-31 23:59 in "UTC").unix() == -60)
  expect((1970-01-01 00:00 in "UTC" + 1500 ms).unix() == 1)
  expect((1970-01-01 00:00 in "UTC" - 1500 ms).unix() == -2)
}

test "the date and time a clock shows" {
  let at = 2026-10-10 23:30 in "UTC"
  expect(at.date(in: "UTC") == 2026-10-10)
  expect(at.date(in: dublin) == 2026-10-11)
  expect(at.time(in: dublin) == 00:30)
  expect(at.date(in: york) == 2026-10-10)
  expect(at.time(in: york) == 19:30)
  let winter = 2026-01-15 08:05 in "UTC"
  expect(winter.time(in: dublin) == 08:05)
  expect(winter.time(in: "Asia/Kolkata") == 13:35)
  expect((1969-12-31 23:59 in "UTC").date(in: "UTC") == 1969-12-31)
  expect((1969-12-31 23:59 in "UTC").time(in: "UTC") == 23:59)
  print("{at.date(in: dublin)} {at.time(in: dublin)} {(at + 45 s).time(in: york)}")
}

test "a date at a time, across the changes of the clocks" {
  expect(2026-10-10.at(07:40, in: dublin) == 2026-10-10 07:40 in dublin)
  expect("{2026-10-10.at(07:40, in: dublin)}" == "2026-10-10 07:40 in \"Europe/Dublin\"")
  # 01:30 doesn't happen in Dublin on 29 March 2026: it is an hour later
  expect("{2026-03-29.at(01:30, in: dublin)}" == "2026-03-29 02:30 in \"Europe/Dublin\"")
  # 01:30 happens twice on 25 October: the second, as Go's time.Date has it
  expect(2026-10-25.at(01:30, in: dublin).unix() == 1792891800)
  expect(2026-10-10.at(23:59, in: dublin) + 3 h > 2026-10-11.at(02:00, in: dublin))
}

test "the next time a clock shows" {
  let at = 2026-10-10 12:00 in dublin
  expect(07:40.next(in: dublin, after: at) == 2026-10-11 07:40 in dublin)
  expect(13:00.next(in: dublin, after: at) == 2026-10-10 13:00 in dublin)
  expect(12:00.next(in: dublin, after: at) == 2026-10-11 12:00 in dublin)
  expect(12:00.next(in: york, after: at) == 2026-10-10 12:00 in york)
  expect(08:00.next(after: at, in: "UTC") == 2026-10-11 08:00 in "UTC")
  print("{01:30.next(in: dublin, after: 2026-03-28 12:00 in dublin)}")
}

test "dates written out" {
  let day = 2026-10-04
  expect(day.format("%A %-d %B %Y") == "Sunday 4 October 2026")
  expect(day.format("%d/%m/%Y") == "04/10/2026")
  expect(2026-10-10.format("%A") == "Saturday")
  expect(1969-12-31.format("%A %d %B") == "Wednesday 31 December")
  expect(day.format("100%% %q %") == "100% q %")
  expect(day.format("%-x %-") == "x ")
  expect(day.format("%H:%M %p") == "00:00 AM")
  let days = [2026-01-01, 2026-02-28, 2026-03-01, 2026-06-15, 2028-02-29]
  print(days |> map(fn d: d.format("%a %A %b %B %-d")) |> join(", "))
}

test "times of day written out" {
  expect(07:05.format("%H:%M") == "07:05")
  expect(07:05.format("%-H:%M") == "7:05")
  expect(19:05:09.format("%H:%M:%S") == "19:05:09")
  expect(19:05.format("%I:%M %p") == "07:05 PM")
  expect(19:05.format("%-I:%M%p") == "7:05PM")
  expect(00:15.format("%-I:%M %p") == "12:15 AM")
  expect(12:00.format("%-I %p") == "12 PM")
  expect(23:59.format("%Y-%m-%d %A") == "0-01-01 Saturday")
  let times = [00:00, 06:30, 11:59, 12:00, 12:01, 23:59:59]
  print(times |> map(fn t: t.format("%H %-H %I %-I %M %S %p")) |> join(" | "))
}

test "a range of times holds a time, wrapping midnight when it ends before it starts" {
  let day = 09:00..17:00
  expect(day |> contains(09:00))
  expect(day |> contains(12:30))
  expect(!(day |> contains(17:00)))
  expect((09:00..=17:00) |> contains(17:00))
  expect(!(day |> contains(08:59)))
  let night = 23:00..08:00
  expect(night |> contains(23:00))
  expect(night |> contains(23:30))
  expect(night |> contains(00:00))
  expect(night |> contains(07:59:59))
  expect(!(night |> contains(08:00)))
  expect((23:00..=08:00) |> contains(08:00))
  expect(!(night |> contains(12:00)))
  expect(night.start == 23:00 && night.end == 08:00)
  print("{night.start} {night.end} {day.start.format("%-H")}")
}

test "a range's ends" {
  let r = 2..=9
  expect(r.start == 2 && r.end == 9)
  let big = 1..100000000000000000000
  expect(big.end == 100000000000000000000)
  let dates = 2026-10-01..2026-10-31
  expect(dates.end == 2026-10-31)
}

test "the digest's and Notify's times" {
  let at = 2026-10-10 06:30 in "UTC"
  expect(irishTime(at) == "10 October 07:30")
  expect(quietUntil(Some(23:00..08:00), at) == Some(2026-10-10 08:00 in dublin))
  expect(quietUntil(Some(23:00..08:00), 2026-10-10 12:00 in dublin) == None)
  expect(quietUntil(None, at) == None)
  print("{quietUntil(Some(22:00..07:00), 2026-10-10 23:00 in dublin)}")
}
