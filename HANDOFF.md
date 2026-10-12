# Handoff, 2026-10-12

What `task next` and ROADMAP.md's "What's next" can't hold: the state of the homelab deploy, and
findings that have no issue yet. Delete an item once it is done or moved to an issue, and the
file once nothing is left.

In flight on 2026-10-12, in worktrees of the coordinating session, to be merged into `dev` by
it: none of the coordinator's own; the helper session builds 7.6's groundwork (the pinned Go
toolchain and the B′ link path). 7.5 is merged and 7.13 mostly (maps and string views left). 7.8, 7.14, G.20 and the audit's fixable bugs (#120 to #130, #133, #137,
#138) are merged; #131, #134, #135 and #136 wait on the owner. If they are merged, `git
worktree list` shows nothing under `.claude/worktrees` and ROADMAP marks them done. If one is
still open with no worktree, it was never merged: start it again from its issue.

## Pick up first

1. **After the digest's first morning run in the homelab** (due 2026-10-12 07:40 Europe/Dublin), read it and
   fix what it shows, as "The digest in the homelab" below says. One good run closes phase 5's
   exit criterion.
2. **After every push to `dev`, check CI** (`gh run list --branch dev --workflow ci.yml --limit
   3`, or the GitHub Actions list for `ci.yml`). A local `task check` isn't enough: CI runs the
   Docker tests (`EYE_TEST_DOCKER=1`) and the VS Code window test, which a laptop or a cloud
   session usually doesn't.

## The native-focus checkpoint

ROADMAP.md's "What's next" opens with a checklist set by the owner on 2026-10-12: when every
box is ticked, all work turns to compiling and the native port (7.6, then 7.11, then 7.12), and
the package manager, open question 41 and new features wait for v0.4. Tick a box in the commit
that finishes it.

## Native compilation: what comes next, in order

The plan the owner accepted on 2026-10-12 is in
[docs/internals/reviews/benchmarks-review-2026-10-12.md](docs/internals/reviews/benchmarks-review-2026-10-12.md)
(read it first) and [native-plan-2026-10-12.md](docs/internals/reviews/native-plan-2026-10-12.md)
(the measurements behind it). Each task below has an issue with its done-when, and a bullet in
ROADMAP.md's phase 7. Take them in this order; a fresh session can start at the first one not
marked done.

1. **7.13 (#117) values laid out by type, with 7.5 (#78) reference counting in the low IR.** The
   big step. The C back end today makes every value one word and everything but small numbers a
   heap object; the low IR is monomorphic, so it can lay values out by type instead: field-less
   enums as integers, small records as C structs, `Option` without a box, lists of fixed-size
   values as arrays, slices that share storage. 7.5 then moves `dup`/`drop` into the low IR, drops
   a value after its last use, infers borrowed parameters and updates in place whenever a value is
   unique. Do the layout first or in the same piece of work: which values are counted depends on
   it. Needs 7.14 merged (it changes the same files).
2. **7.9 (#112) core gaps in C:** time methods, `toJson`, the `Schema`/`Save` derivations,
   `random.*`, `http.get`. What the lab needs beyond entities and jobs.
3. **7.10 (#113) crossings: done.** `eye run --crossings` and `eye test --crossings` count them; a
   digest morning makes 214 (about 7.5 µs under B′), and none is in a hot loop, so nothing needs
   batching.
4. **7.6 (#79) the runner hosts compiled programs, with B′ hosting** (decided): the Go runner is
   the program's `main`, `eye build` links the program's C into it through cgo with the pinned
   zig as C compiler and a pinned Go toolchain fetched like zig. One static binary. Done when the
   morning digest runs as a native job on the journal and on Postgres, with idle memory and
   resume time recorded beside the interpreter's. **This is v0.4's exit test.**
5. **7.11 (#114) faster than Go:** the owner's bar, per benchmark, tracked throughout by
   `go run ./tools/bench --check`; closed when native code beats Go on time on every benchmark and
   on memory for every program without entities or jobs.
6. **7.12 (#115) reuse and fault regions:** a `match` arm reuses the matched cell; a failing test
   frees what it allocated.
7. **After v0.4: S.2 (#116) the runner in Eyelang,** so a job program has no Go in it and can beat
   Go on memory.

Rules for every native task: run `go run ./tools/bench --check` before and after (it fails when a
ratio to Go gets worse), record gains with `go run ./tools/bench --write` on a quiet machine,
keep `go test ./cgen`'s cross-check against the interpreter green, and update the benchmark
review's numbers when they change a conclusion.

Outside the compiler, and not for this repository alone:
- **The digest in the homelab** (below): read its runs, fix what they show, move its pin.
- **Phase 6's exit test:** 6.6 (Notify in the homelab repository) and 6.8 (sessions with real
  coding agents).

Waiting on the owner:
- **The package manager's first step (P0)**, recommended in
  [performance-and-packages-2026-10-12.md](docs/internals/reviews/performance-and-packages-2026-10-12.md),
  section 2.4: `[dependencies]` in `eye.toml` with `path` and `git` sources, `eye.lock` with
  content hashes, a cache and `eye vendor`. No task until the owner says go.
- **Open question 41:** runs pinned to the code they started on.
- Whether pipelines over entity sets officially run in memory until several runners share a
  database (ROADMAP, "Owed by releases already made").

## The digest in the homelab

The Eyelang digest runs in the homelab cluster in shadow mode, beside the TypeScript digest.
Session eyelang-af did the cluster work in the stacks-homelab repository.

- Settings: `DIGEST_EVENT=digest.ireland.eye`, `DIGEST_TO` set to the operator only, and the
  store in schema `eyelang` of the platform `digest` database. `AI_PRICES=gpt-5.6-terra=1.25/10`
  was approved by the user.
- Two trials ran. The first failed on hard-coded Miniflux category ids. The second sent seven
  empty sections. Both are fixed on dev (89af4f7):
  - categories are found by title and created when missing;
  - all seven sections of the TypeScript digest are ported;
  - the model is told today's date and the window;
  - an empty briefing from non-empty input ends as `EmptyBriefing(items: N)`, not "Nothing new".
- Checked against the real services: the gateway passes `response_format` json_schema strict
  through to gpt-5.6-terra, and the seven "Digest: …" category titles match.
- Deployed: eyelang 89af4f7, at stacks-homelab a0ab0b12f (CI 4423 passed). The pod
  `digest/eyelang-digest-5b498d48d4` is healthy with no restarts. `AI_PRICES`, internet egress
  (the pod label `https`) and the README are rolled out. Its startup line names the store
  (`…/digest schema eyelang`), 384 MiB and the next morning run, 2026-10-12 07:40 Europe/Dublin.
  The startup database retry worked live: DNS was refused for the first seconds, then it
  connected.
- A third trial at 14:54 (a one-off pod with an in-memory store) sent a real 6.8 KB digest to the
  operator only: a strike highlight, 5 Ireland items, 1 under Money and housing. Its direct pages
  all failed, because that trial pod lacked the egress label; the deployed pod has it. Two sources
  fail for the TypeScript digest too: the Immigrant Council of Ireland feed ("unable to detect
  feed format"), and the Embassy page, blocked by Cloudflare.
- To see what happened at 07:40: `eye runs /program` lists the runs, and `eye runs /program ID`
  lists a run's steps. On the deployed 89af4f7, values are cut to 100 characters. From 111a64e
  on, two more commands help:
  - `eye runs /program ID STEP` shows one step in full, with a model call's reply and how it was
    sent;
  - `--show-replies` prints every reply.
- **Risk:** in a new pod, network rules take effect a few seconds after the container starts.
  Contract calls (Miniflux, Notify) retry for about a minute, but `http.get` and `ai.generate`
  fail at once (`Err(NoAnswer)`, `Err(ProviderError)`). A 07:40 run missed while the pod was down
  runs right after start, inside that window. Until the runtime retries those calls too, don't
  restart the pod near 07:40, or delay startup in the pod spec. On dev the runtime now does
  (2026-10-12): both try a service they can't reach again for about a minute. The risk goes once
  the deployed pin moves past that commit.
  eyelang-af doesn't push to the stacks repository between 06:30 and 08:30 Irish time, so the
  Deployment doesn't change near the run.
- After the first 07:40 run (2026-10-12), check three things:
  - `eye runs /program` shows the morning run as `Ok(Accepted(…))`;
  - the direct pages were read;
  - the Telegram message arrived.

  Then move the pin to the latest dev, which adds the model-call details in `eye runs` (111a64e).
- Phase 5's exit criterion ("the morning digest runs in the lab") closes after one good morning
  run. Then ROADMAP's "Owed by releases already made" drops that item.
- Replacing the TypeScript digest, instead of running beside it, is the user's decision.
- **Digest fix owed:** a "Sources not read" line wraps its error twice and escapes it, as in
  `NoAnswer(reason: "NoAnswer(reason: \"Get …\")")`. It should read like
  "<source>: <host> refused the connection". Fixed on dev (2026-10-12): `web.FetchError` shows
  its reason, and `http.get`'s says why in words; the deployed pin still has the old line.

## Findings without an issue yet

The core gaps that self-hosting needs are issues now (G.1 to G.6, #84 to #89), and so is serving
a contract over HTTP (6.13, #81). Open questions 32 to 34 (time-travel debugging, effects of stored functions, an
online playground) are in SPECS.md.

Runtime and tools:
- The docs audit of 2026-10-12 replaced session eyelang-15's lost list with issues #120 to #139 (the restart of a failed run is #136; #132 and #139 were fixed on `dev` the same day).

Checker:
- A generic private function calling itself at a bigger type is reported by `eye check --lir`
  (7.2), not by `eye check`.
- `check/resolve_test.go`'s unresolvable list keeps `nick` and `nickname` on purpose: a
  migration reads the old row by name, which nothing declares.

Language server and editor:

Syntax: open questions 13 (`eye fmt` and the two forms of `if`), 14 (`if let`), 25 (naming a
record's type at its literal) and 31 (`Option` parameters with defaults) each need a decision
before work starts.

Lab:
- `trains.watch` (`testdata/lab/trains/trains.eye`) is a `while true` loop that sleeps 45 s. Since
  checkpoints (7.3), each wake replays only the steps since the statement it is in, so its record
  stays small. The homelab gets this when its pin moves past the checkpoint commit; runs saved
  before it replay once, then keep checkpoints.
