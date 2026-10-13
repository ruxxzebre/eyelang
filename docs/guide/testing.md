---
title: Testing with time and recordings
---

# Testing with time and recordings

Programs that keep state and wait are hard to test in most languages. The test has to wait for
real time to pass, or mock the clock in every place that reads it. It talks to real services, or to
mocks that drift from what the services do. It shares a database with other tests, and fails when
they run in a different order. Such tests are slow, and they fail now and then for no reason.

Eyelang tests don't have these problems, by design: each test gets its own empty store, a clock
that moves only when the test says so, and recorded answers in place of other services. A test of
three days of waiting runs in a millisecond, and gives the same result every time.

The code on this page is a [real program](https://github.com/ruxxzebre/eyelang/tree/dev/docs/guide/code/testing)
that the test suite checks and runs: a morning digest, sent through a chat service at 07:30.

[Open it in the playground](/playground#example=guide/testing) to run it, test it and change it in your browser.

## Test blocks

A `test` block is a declaration, next to the code it tests. It has a name and a body, and
`expect(condition)` fails it:

<<< ./code/testing/format/format.eye#headline

`eye test` runs every test in the program, the tests of the modules it imports from its own
directory included:

```sh
eye test docs/guide/code/testing
```

```text
ok    format: a headline names the day and counts the items (11µs)
ok    the digest goes out at 07:30 (268µs)
ok    each test starts with an empty store (2µs)
ok    a lost reply is settled by asking Notify, not by sending again (125µs)
ok    a refused digest keeps the items for tomorrow (81µs)
ok    a run that passes its deadline ends as an outcome (146µs)
6 tests passed (633µs)
```

A test from another module has the module's name in front, here `format:`. When a comparison
fails, `expect` shows the value it got, and a failing test shows what it logged. Here two
expectations were changed to be wrong:

```text
ok    format: a headline names the day and counts the items (154µs)
FAIL  the digest goes out at 07:30 (1.914ms)
      main.eye:46:3: expected Inbox("ann").sent == ["m-9"], but Inbox("ann").sent is ["m-1"]

ok    each test starts with an empty store (3µs)
ok    a lost reply is settled by asking Notify, not by sending again (121µs)
FAIL  a refused digest keeps the items for tomorrow (84µs)
      main.eye:73:3: expected Inbox("ann").items == [], but Inbox("ann").items is ["Bin day moved"]

      warn: Notify refused the digest for 2026-10-14: no chat for ann
ok    a run that passes its deadline ends as an outcome (498µs)
2 of 6 tests failed
```

`eye test` exits with status 1 when a test fails, so CI can run it as it is.

- `--run text` runs only the tests whose names contain the text:

  ```text
  $ eye test --run lost
  ok    a lost reply is settled by asking Notify, not by sending again (242µs)
  1 tests passed (242µs)
  ```

- `-v` shows what passing tests logged, too:

  ```text
  $ eye test -v --run refused
  ok    a refused digest keeps the items for tomorrow (217µs)
        warn: Notify refused the digest for 2026-10-14: no chat for ann
  1 tests passed (217µs)
  ```

## A fresh store for each test

Each test starts with an empty, in-memory store: no entities, no runs, no events. What one test
writes, the next can't see, so tests can run in any order and don't need set-up or clean-up code:

<<< ./code/testing/main.eye#test-fresh

The test before this one sent a digest and changed `Inbox("ann")`. Here it is new again. Since an
[entity](/guide/entities) exists from its first use, a test creates the state it needs by calling
methods, as a user of the program would.

## Time

The program sends a digest every morning:

<<< ./code/testing/main.eye#inbox

<<< ./code/testing/main.eye#digest

In a test, the clock starts at 2026-01-01 00:00 UTC, or at the time `with clock(...)` gives, and
moves only when the test calls `advance(...)`:

<<< ./code/testing/main.eye#test-digest

- `with clock(...)` and `with fakes(...)` apply to the rest of the block. `with` works only in
  tests.
- `advance(89 min)` moves the clock to 07:29, and nothing has fired yet. `advance(1 min)` reaches
  07:30. The schedule fires, the run sends the digest, and all of it finishes before `advance`
  returns, so the next line sees the result.
- Each run that wakes on the way runs in the order of its wake time, and so does each schedule that
  fires, if its job is in the test's own module.
- A run started by a call, from a method or a test, goes as soon as the call returns, until it
  finishes or waits.

So a test reads like the story it checks, and a day passes in a line.

## Other services: fakes and recordings

The digest calls Notify, which is another service, described by a contract:

<<< ./code/testing/notify/notify.eye#contract

In a test, `with fakes(notify, from: "recordings/")` answers Notify's calls from
`recordings/notify.json`, a file of real exchanges:

<<< ./code/testing/recordings/notify.json

- A recording is a JSON file per contract module, named after the module, with the contract's name
  and a list of exchanges.
- Each exchange names an operation, the key the call was made with, and the result, in the same
  JSON shape the service uses: a variant is an object with a `"kind"`.
- A call is answered by the exchange with its operation and key. A call without a key gets the
  first exchange for its operation.
- `{ "kind": "Uncertain", "key": "…" }` records a command whose reply was lost.

The second exchange is such a lost reply. The test for it checks that the program settles it by
asking `sent` with the same key, and doesn't send the digest twice:

<<< ./code/testing/main.eye#test-lost

The fourth exchange is a refusal, and its test checks that the items wait for the next day:

<<< ./code/testing/main.eye#test-refused

Recordings are checked against the contract when the test loads them, so they can't drift from it
unnoticed. A result that the contract's types don't allow fails the test:

```text
FAIL  a refused digest keeps the items for tomorrow (101µs)
      main.eye:68:3: recordings/notify.json: exchange 4 (send): Delivery has no kind "Rejected"
```

A call that no exchange answers fails the test, and so does a call to a service the test has no
fakes for, even when the service's address is set: a test doesn't reach the network unless
`eye test --live` names the module. This holds for a contract's operations, `http.get` and
`ai.generate` alike:

```text
main.eye:31:9: job digest: recordings/notify.json has no exchange for notify.send with key "digest-2026-10-15"
main.eye:31:9: job digest: a test doesn't reach the network: notify.send has no fake here; add with fakes(notify, from: "recordings/"), or run eye test --live notify
```

### What the recordings show

The recordings are also what `eye check` trusts about a service. Reading a field of the service's
answer that no recording has is an error, since nothing shows the service ever sends it:

```text
main.eye:54:13: Refused's field error is in no recording of Chat (recordings/chat.json), so nothing shows the service sends it: record an answer that has it, or mark where it is read @unwitnessed("why")
```

- The recordings that count are the ones the tests replay: each `fakes(m, from: "dir/")` names
  `dir/m.json`, beside the file it is written in.
- A read is a field access, `r.receipt`, or a pattern that looks inside a field,
  `Refused(error)`. Matching a variant with `Refused(_)` or `Refused(..)` reads nothing, so a
  match can handle a variant no recording has.
- When the author knows a field exists but has never caught it, as a refusal that happens once a
  year, they mark the function, job or test that reads it, or the type,
  `@unwitnessed("Chat refuses only when the channel is archived")`.
- A contract no test replays recordings of isn't checked, and neither is one the program serves
  itself.

Model calls are faked the same way: `with fakes(ai, from: "recordings/")` reads
`recordings/ai.json`, whose exchanges are the replies, used in order. See [AI calls](/guide/ai-calls).

Programs the code runs are too: `with fakes(process, from: "recordings/")` answers `process.run`
from `recordings/process.json`, one exchange per command line, such as `"key": "git pull
--ff-only"`, with what the run returns (`{ "kind": "Ok", "value": { "status": 0, "stdout": "…",
"stderr": "" } }`). An exchange without a key answers every other command, and
`ran.calls(process.run)` lists the runs, each with its `program`, `args`, `input` and `dir`.

### Running against the real services

`eye test --live ai,notify` runs the same tests against the real model and the real Notify. A
`with fakes(…)` leaves the modules named out, and their calls go where `eye run` sends them, with
the address and secret from the environment: `NOTIFY_URL` and `NOTIFY_TOKEN` for Notify, and
`AI_URL`, `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` for a model. The other modules' fakes keep
replaying, and `sent.calls(op)` still lists what was sent.

A test whose service has no address or secret set is skipped, with what to set. A skipped test
doesn't fail the run, so `--live` works on a laptop that has some keys and not others:

```text
ok    a sent digest is not sent again (2ms)
skip  the digest is written by a model
      --live ai: no model provider is set: set AI_URL, ANTHROPIC_API_KEY or OPENAI_API_KEY
3 tests passed, 1 skipped (12ms)
```

### Making a recording

`eye test --live notify --record` writes what the real service answered as the recording the
`with fakes(…)` reads, and names each file it writes. The next plain `eye test` replays it:

```text
ok    a sent digest is not sent again (240ms)
ok    a refused digest keeps the items for tomorrow (180ms)
recorded recordings/notify.json (2 exchanges)
2 tests passed (420ms)
```

- Only a test that passed writes. A failed or skipped test writes nothing.
- A contract's recording keeps one exchange per operation and key, and the exchanges of other
  operations and keys stay, so `--run` re-records one test without losing the others' answers.
- `ai.json` holds one test's replies in order, and is replaced.
- The value of every secret the test read is written as `<secret name>`, even inside an answer
  that repeats it. A recording never holds a request or its headers.

The file is updated in place: review the change with the rest of the diff. You can also write a
recording by hand, from a reply you saw in a service's logs. Keep one directory per situation when
tests need different answers to the same call, and name it in `from:`, as the tutorial's
[approvals](/tutorial/approvals) program does for a model that fails.

## Outcomes, approvals and events

A run that stops on a bug fails its test: `unreachable`, a fake with no answer, a recording that
doesn't fit. The failure says where the run stopped, as the messages above do.

A run that ends as an outcome the program declared doesn't fail the test. A run past its
`@deadline` ends as `Failed(DeadlinePassed)`, which is part of how the program behaves. To check
it, read the run's outcome. A test can't wait, so it doesn't join the run: `run.outcome()` gives
`None` while the run is still going, and `Some` of how it ended once the clock has moved past its
end:

<<< ./code/testing/main.eye#deadline

<<< ./code/testing/main.eye#test-deadline

Three more things stand in for the world outside:

- **People:** `approve(text)` and `decline(text)` answer the one `approval!` whose text contains
  the given text, as the person it asks; `by:` answers as someone else, whose answer is refused.
  They fail the test when no waiting approval, or more than one, contains the text. See
  [Jobs and waiting](/guide/jobs-and-waiting#ways-to-wait).
- **Events:** `contract.event.publish(...)` stands in for another service sending an event, and
  the runs it starts or wakes go before it returns. See
  [Views, events and schedules](/guide/views-events-schedules#events).
- **The machine's memory:** a test's runner has 1 GiB, and `with memory(64 MiB)` gives it that
  much instead, for the rest of the test. `@memory` and `reserve!` are admitted against it, so a
  test shows a run waiting for memory, or ending as `Failed(OutOfMemory)`, on any machine. Runs
  that wait for memory and fit under a larger limit run before the next line. See
  [Time, secrets, tests](/reference/language/time-secrets-tests).

## Why tests can't flake

A flaky test passes or fails depending on something it doesn't control. Eyelang tests control all
of it:

| Source of flakiness | In an Eyelang test |
|---|---|
| Real time passing | The clock moves only with `advance`, and starts where `clock(...)` says |
| The host's time zone | Zones are named in the code, never taken from the machine |
| A slow or down service | Calls are answered from recordings; a missing one fails the test |
| A model's different answer each time | Model replies are recorded too |
| State left by another test | Each test has its own empty store |
| Runs and tasks in a different order | On the test clock, runs wake in order of their wake time, and tasks take turns |

The same test gives the same result on every run, on any machine.

## Mistakes this prevents

| The bug | Why it can't happen |
|---|---|
| A test that sleeps, and is slow | There is no real time in a test; `advance` jumps |
| A mock that agrees with the code but not with the service | Fakes are real exchanges, checked against the contract |
| A test that calls production | A call with no recorded answer fails |
| Test-only switches left in the program | `with` works only in tests: `with applies a test handler, such as clock(…) or fakes(…), and works only in tests` |
| Tests that pass alone and fail together | Each test has its own store |

## See also

- [Time, secrets, tests](/reference/language/time-secrets-tests) in the reference: the rules for
  clocks, fakes and outcomes in tests.
- [The eye command](/reference/cli#test): `eye test` and its options.
- [Talking to other services](/guide/contracts): contracts, keys and `Uncertain`.
- [Jobs and waiting](/guide/jobs-and-waiting) and [Entities](/guide/entities): what the tests
  above exercise.
- [Built-in functions](/reference/api/std/builtins#tests): `expect`, `advance`, `clock`, `memory`,
  `approve`, `decline` and `fakes`.
