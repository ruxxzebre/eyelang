# The limits a job declares, as the runtime enforces them: @concurrency,
# @limit and @deadline, and runs started once per key.

## Counts the runs going at once, and the order they began in.
entity Meter(name: String) {
  state running: Int = 0
  state peak: Int = 0
  state order: List(String) = []

  pub fn enter(who: String) {
    running += 1
    if running > peak { peak = running }
    order += [who]
  }

  pub fn leave() { running -= 1 }
}

@concurrency(1)
job one(who: String) {
  Meter("one").enter!(who)
  sleep!(1 min)
  Meter("one").leave!()
}

## One at a time for each chat; chats don't wait for each other.
@concurrency(1, by: chat)
job perChat(chat: String, n: Int) {
  Meter(chat).enter!("{chat}-{n}")
  sleep!(1 min)
  Meter(chat).leave!()
}

## Starts at most once a second in each chat.
@limit(1, per: 1 s, by: chat)
job send(chat: String, n: Int) {
  Meter("send").enter!("{chat}-{n}")
}

@deadline(10 min)
job slow() -> String {
  sleep!(1 h)
  "done"
}

job watcher() -> String {
  match slow.start!().join!() {
    Completed(text) => text
    Failed(fault) => "{fault}"
  }
}

job report(text: String) {
  Meter(text).enter!(text)
}

job waitForSlow() {
  let outcome = watcher.start!().join!()
  match outcome {
    Completed(text) => report.start!(text)
    Failed(_) => report.start!("watcher failed")
  }
}

@on(every(1 min))
@concurrency(1)
job sweep(day: Date) {
  Meter("sweep").enter!("{now!().time(in: "UTC")}")
  sleep!(150 s)
  Meter("sweep").leave!()
}

test "@concurrency(1) runs one at a time, in the order they were started" {
  for who in ["a", "b", "c"] { one.start(who) }
  expect(Meter("one").peak == 1)
  advance(10 min)
  expect(Meter("one").order == ["a", "b", "c"])
  expect(Meter("one").peak == 1 && Meter("one").running == 0)
}

test "by: limits each group on its own" {
  perChat.start("x", 1)
  perChat.start("x", 2)
  perChat.start("y", 1)
  expect(Meter("x").order == ["x-1"])
  expect(Meter("y").order == ["y-1"])
  advance(1 min)
  expect(Meter("x").order == ["x-1", "x-2"])
  expect(Meter("x").peak == 1)
}

test "@limit spaces starts in each group" {
  send.start("x", 1)
  send.start("x", 2)
  send.start("y", 1)
  expect(Meter("send").order == ["x-1", "y-1"])
  advance(1 s)
  expect(Meter("send").order == ["x-1", "y-1", "x-2"])
}

test "a run past its @deadline fails, even asleep, and whoever joins it sees why" {
  waitForSlow.start()
  advance(9 min)
  expect(Meter("DeadlinePassed").order == [])
  advance(2 min)
  expect(Meter("DeadlinePassed").order == ["DeadlinePassed"])
}

test "a scheduled firing that finds the limit reached is skipped" {
  advance(5 min)
  # 150 s runs, one at a time, firing every minute: the firings at 00:02 and
  # 00:03 come while one is going, and are skipped rather than queued, so the
  # next run starts on the minute, at 00:04.
  expect(Meter("sweep").order == ["00:01", "00:04"])
  expect(Meter("sweep").peak == 1)
}

test "a run starts once per key" {
  let first = one.start("k", key: "only")
  let second = one.start("k", key: "only")
  expect(first == second)
  advance(10 min)
  expect(Meter("one").order == ["k"])
}
