# Guide: views, events and schedules. Disks report how full they are; a view
# finds the nearly full ones; jobs page someone and wait for an answer; a
# schedule sends a morning summary.

import monitor.{Reading}

let dublin: Zone = "Europe/Dublin"

## Who was told what, so the tests can look. A real program would send these
## lines through a contract.
entity Pager(name: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

# region disk
entity Disk(host: String) {
  state used: Int = 0

  pub fn report(percent: Int) { used = percent }

  ## Someone has seen the alert: tell every run that waits for it.
  pub fn acknowledge(by: String) {
    monitor.acks.publish({ host, by })
  }
}
# endregion disk

# region record
## Each reading the monitor sends starts one run, and updates its disk.
@on(event(monitor.readings))
job record(r: Reading) {
  Disk(r.host).report!(r.used)
}
# endregion record

# region view
## Whether a disk is 90% full or more.
fn full(d: Disk) -> Bool = d.used >= 90

## Every disk that is nearly full.
view nearlyFull = Disk.all |> filter(full)
# endregion view

# region triggers
## Pages the on-call person when a disk becomes nearly full, and tells the
## lead if nobody acknowledges it within an hour.
@on(enters(nearlyFull))
job page(d: Ref(Disk)) {
  let host = d.host
  Pager("on-call").add!("{host} is nearly full")
  let ack = next!(monitor.acks, where: fn a: a.host == host, within: 1 h)
  match ack {
    Some(a) => Pager("on-call").add!("{a.by} is on {host}")
    None => Pager("lead").add!("nobody answered for {host}")
  }
}

## Says when a disk is back to normal.
@on(leaves(nearlyFull))
job settle(d: Ref(Disk)) {
  Pager("on-call").add!("{d.host} is fine again")
}
# endregion triggers

# region test-view
test "a disk that fills up enters the view, and leaves it when cleaned" {
  monitor.readings.publish({ id: 1, host: "nas", used: 80 })
  expect(Pager("on-call").lines == [])
  monitor.readings.publish({ id: 2, host: "nas", used: 93 })
  monitor.readings.publish({ id: 3, host: "nas", used: 95 })
  expect(Pager("on-call").lines == ["nas is nearly full"])
  Disk("nas").acknowledge("ann")
  monitor.readings.publish({ id: 4, host: "nas", used: 60 })
  expect(Pager("on-call").lines == ["nas is nearly full", "ann is on nas", "nas is fine again"])
}
# endregion test-view

# region test-events
test "a reading sent twice starts one run" {
  monitor.readings.publish({ id: 1, host: "nas", used: 95 })
  monitor.readings.publish({ id: 1, host: "nas", used: 95 })
  expect(Pager("on-call").lines == ["nas is nearly full"])
}

test "nobody acknowledges, so the lead hears about it" {
  monitor.readings.publish({ id: 1, host: "nas", used: 95 })
  advance(1 h)
  expect(Pager("lead").lines == ["nobody answered for nas"])
}
# endregion test-events

# region morning
## On weekday mornings, a summary of the disks that are nearly full.
@on(weekdays(08:30, in: dublin))
job morning(day: Date) {
  let hosts = Disk.all |> filter(full) |> map(fn d: d.host)
  if hosts == [] { return }
  Pager("team").add!("{day.format("%A")}: {hosts |> join(", ")} nearly full")
}
# endregion morning

# region compact
## Compacts the disks every night at 01:30, Irish time.
@on(daily(01:30, in: dublin))
job compact(day: Date) {
  Pager("night").add!("{day} at {now!()}")
}
# endregion compact

# region test-schedules
test "the summary comes on weekday mornings only" {
  with clock(2026-10-16 08:00 in dublin)  # a Friday
  Disk("nas").report(97)
  advance(3 days + 30 min)
  expect(Pager("team").lines == ["Friday: nas nearly full", "Monday: nas nearly full"])
}

test "a time the clock skips runs as far after the change" {
  with clock(2027-03-27 12:00 in dublin)  # at 01:00 on the 28th, the clock jumps to 02:00
  advance(2 days)
  expect(Pager("night").lines == [
    "2027-03-28 at 2027-03-28 02:30 in \"Europe/Dublin\"",
    "2027-03-29 at 2027-03-29 01:30 in \"Europe/Dublin\"",
  ])
}
# endregion test-schedules
