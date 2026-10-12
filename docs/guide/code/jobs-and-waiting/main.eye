# Guide: jobs and waiting. Reminders that ring at their time, exports that
# must finish by a deadline, and a clean-up that asks a person first.

## What the runs wrote down, so the tests can look. A real program would send
## these lines through a contract instead.
entity Log(name: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

# region reminder
entity Reminder(id: String) {
  state text: String = ""
  state due: Instant = 2026-01-01 00:00 in "UTC"

  pub fn set(t: String, at: Instant) {
    text = t
    due = at
    ring.start(id, key: "ring-{id}")
  }

  pub fn snooze(by: Duration) { due = due + by }
}
# endregion reminder

# region ring
## Rings a reminder when it is due. A snooze moves the time while the run
## sleeps, so it reads the reminder again after every wait.
job ring(id: String) {
  var me = Reminder(id).read!()
  while me.due > now!() {
    sleepUntil!(me.due)
    me = Reminder(id).read!()
  }
  Log("rang").add!("{me.text} at {now!().time(in: "Europe/Dublin")}")
}
# endregion ring

# region test-ring
test "a reminder rings at its time, and a snooze moves it" {
  with clock(2026-10-12 09:00 in "Europe/Dublin")
  Reminder("tea").set("put the kettle on", 2026-10-12 10:00 in "Europe/Dublin")
  Reminder("call").set("call the plumber", 2026-10-12 11:00 in "Europe/Dublin")
  advance(1 h)
  expect(Log("rang").lines == ["put the kettle on at 10:00"])
  Reminder("call").snooze(30 min)
  advance(2 h)
  expect(Log("rang").lines == ["put the kettle on at 10:00", "call the plumber at 11:30"])
}
# endregion test-ring

# region test-key
test "setting a reminder twice starts one run" {
  with clock(2026-10-12 09:00 in "Europe/Dublin")
  let r = Reminder("tea")
  r.set("put the kettle on", 2026-10-12 10:00 in "Europe/Dublin")
  r.set("put the kettle on, now", 2026-10-12 10:15 in "Europe/Dublin")
  advance(1 day)
  expect(Log("rang").lines == ["put the kettle on, now at 10:15"])
}
# endregion test-key

# region prune
## Deletes old backups, but only if a person says yes within a day.
job prune(old: Int) {
  match approval!("delete {old} old backups", from: "operator", within: 1 day) {
    Approved(ok) => Log("backups").add!("deleted {old}, approved by {ok.by}")
    Declined => Log("backups").add!("kept {old}: declined")
    Expired => Log("backups").add!("kept {old}: nobody answered")
  }
}
# endregion prune

# region test-prune
test "a person approves the clean-up" {
  prune.start(3)
  expect(Log("backups").lines == [])
  approve("delete 3 old backups", by: "someone else")
  expect(Log("backups").lines == [])  # only the person asked may answer
  approve("delete 3 old backups")
  expect(Log("backups").lines == ["deleted 3, approved by operator"])
}

test "nobody answers, so nothing is deleted" {
  prune.start(3)
  advance(1 day)
  expect(Log("backups").lines == ["kept 3: nobody answered"])
}
# endregion test-prune

# region export
type Table = { name: String, minutes: Int }

## Exports one table. The sleep stands in for slow work in another service.
@deadline(30 min)
job export(t: Table) -> Int {
  sleep!(1 min * t.minutes)
  t.minutes * 1000
}

## Starts one export per table, then waits for each. The exports run at
## once; one that passes its deadline is reported, not lost.
job nightly(tables: List(Table)) {
  var runs = []
  for t in tables { runs += [(t.name, export.start!(t))] }
  for (name, run) in runs {
    match run.join!() {
      Completed(rows) => Log("nightly").add!("{name}: {rows} rows")
      Failed(fault) => Log("nightly").add!("{name}: {fault}")
    }
  }
}
# endregion export

# region test-export
test "child runs go at once, and a fault comes back as an outcome" {
  let start = now()
  nightly.start([{ name: "users", minutes: 5 }, { name: "events", minutes: 45 }, { name: "logins", minutes: 20 }])
  advance(1 h)
  expect(Log("nightly").lines == ["users: 5000 rows", "events: DeadlinePassed", "logins: 20000 rows"])
  expect(now() - start == 1 h)
}
# endregion test-export

# region post
## Posts to a chat. The chat service allows one message a second in each
## chat, and the runtime keeps to that.
@limit(1, per: 1 s, by: chat)
job post(chat: String, text: String) {
  Log(chat).add!(text)
}

## Rebuilds a chat's search index, one rebuild at a time per chat.
@concurrency(1, by: chat)
job reindex(chat: String) {
  Log("reindex").add!("start {chat}")
  sleep!(10 min)
  Log("reindex").add!("end {chat}")
}
# endregion post

# region test-post
test "@limit spaces runs out, per chat" {
  post.start("team", "one")
  post.start("team", "two")
  post.start("family", "hello")
  expect(Log("team").lines == ["one"] && Log("family").lines == ["hello"])
  advance(1 s)
  expect(Log("team").lines == ["one", "two"])
}

test "@concurrency queues runs, per chat" {
  reindex.start("team")
  reindex.start("team")
  reindex.start("family")
  advance(10 min)
  expect(Log("reindex").lines == ["start team", "start family", "end team", "start team", "end family"])
}
# endregion test-post

# region main
## Sets one reminder the first time the program starts.
pub fn main() uses {entity Reminder, clock} {
  let r = Reminder("stretch")
  if r.read().version == 0 {
    r.set("stand up and stretch", now() + 1 min)
  }
}
# endregion main
