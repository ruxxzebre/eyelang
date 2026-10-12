# Handoff, 2026-10-12

What `task next` and ROADMAP.md's "What's next" can't hold: the state of the homelab deploy, and
findings that have no issue yet. Delete an item once it is done or moved to an issue, and the
file once nothing is left.

Nothing is in flight: every branch of 2026-10-11 and 2026-10-12 is merged into `dev`, no
worktree is left, and nobody holds a claim.

## Pick up first

1. **Read the digest's first morning run in the homelab** (2026-10-12 07:40 Europe/Dublin) and
   fix what it shows, as "The digest in the homelab" below says. One good run closes phase 5's
   exit criterion.
2. **After every push to `dev`, check CI** (`gh run list --branch dev --workflow ci.yml --limit
   3`, or the GitHub Actions list for `ci.yml`). A local `task check` isn't enough: CI runs the
   Docker tests (`EYE_TEST_DOCKER=1`) and the VS Code window test, which a laptop or a cloud
   session usually doesn't.

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
  restart the pod near 07:40, or delay startup in the pod spec.
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
  "<source>: <host> refused the connection".

## Findings without an issue yet

The core gaps that self-hosting needs (fixed-width integers, character literals as `Byte`,
`iter`, `Type.f()` through a module, what a task captures) are listed in ROADMAP's "What's
next", item 1. Open questions 32 to 34 (time-travel debugging, effects of stored functions, an
online playground) are in SPECS.md.

Runtime and tools:
- `eye run` doesn't serve a program's `impl Contract` over HTTP on its `@port`: a served
  contract is called only from inside its own program. 6.4's round trip and the playground's
  contracts demo need it; no task holds it.
- `http.get` and `ai.generate` should retry an unreachable service the way contract calls do
  (see the risk above).
- `ai.generate` has no `reasoning_effort` parameter; the TypeScript digest sends "medium".
- Replacing a key's value in a map (`m |> insert(k, f(m[k]))`, counting words) still copies the
  map, so a loop that updates one map is quadratic. Adding a set's element or a map's new key
  doesn't: they share a `shelf` the way lists share a buffer (`interp/value.go`). String building
  is most of what is left of `plain()`'s 0.47 s.
- A faulted run stays failed, and nothing restarts it (docs/guide/running.md, "Not yet"). It
  needs a command, such as `eye runs --retry ID`, and a decision first: whether the retried run
  replays the steps it recorded before the fault.
- `fakes(…)` returns the plain `Handler` type, so `clock(…).calls(op)` fails only at run time; a
  separate `Fakes` type would catch it.
- The tutorial and guide writers hit about 20 checker and interpreter bugs on 2026-10-11, most
  passing `eye check` and failing at run time. Session eyelang-15 holds the list; some are fixed.
  It should become issues or tests.

Checker:
- A command that returns a record can't have `Uncertain` matched. The lab's `createCategory`
  wraps its result in a one-variant type to work around it.
- `consume` isn't inferred from `@on(event(…))`, because a job has no `uses` clause.
- Generic private functions: `double(2.5)` gives two errors where one would do. Calling itself
  at a bigger type is reported by `eye check --lir` (7.2), not by `eye check`.
- `eye fix` doesn't add the `!` that `publish!` and `start!` now need in jobs.
- `check/resolve_test.go`'s unresolvable list keeps `nick` and `nickname` on purpose: a
  migration reads the old row by name, which nothing declares.

Language server and editor:
- Hover could show a private function's inferred effects.
- The checker could skip only the declaration with a syntax error, so type errors, hover and
  completion keep working while a file is half typed.
- CI could cache `editors/vscode/node_modules` by `package-lock.json` (about 4 s).

Library:
- No hour-and-minute format for a time of day; the digest takes the first five characters of
  its text.
- Not built: `fs.lines`, reading a file as bytes, fakes for `process`, regular expressions. The
  digest's `plain()` strips HTML without them.

Syntax: open questions 13 (`eye fmt` and the two forms of `if`), 14 (`if let`), 25 (naming a
record's type at its literal) and 31 (`Option` parameters with defaults) each need a decision
before work starts.

Lab:
- `trains.watch` (`testdata/lab/trains/trains.eye`) is a `while true` loop that sleeps 45 s. Since
  checkpoints (7.3), each wake replays only the steps since the statement it is in, so its record
  stays small. The homelab gets this when its pin moves past the checkpoint commit; runs saved
  before it replay once, then keep checkpoints.
