# Handoff, 2026-10-11

Where the parallel work of 2026-10-11 stopped, and what to pick up first. `task next` and
ROADMAP.md's "What's next" stay the source of truth for the plan; this file holds what they
can't: branches in flight, findings that have no issue yet, and the state of the homelab deploy.
Delete it once its items are done or moved to issues.

Every lane that ran on 2026-10-11 is merged into dev, `track/runtime-wip` last (see "CI and
checks" for how to confirm CI).

## Pick up first

1. **Done: `track/runtime-wip` is merged.** Its first four commits were already on dev, rebased
   (c563080 to 9ec971b). The other three were cherry-picked: Q16 and #64, `fakes(…).calls(op)`,
   and `eye run --fresh`. `check/infer.go`'s `calls` map holds one record type per operation,
   built from the contract's declared parameter types, so generic private functions don't touch
   it. `TestFakeCalls` checks its error message. The digest tests that dev added since then ask
   the fake what was sent, not `Day.sent`. Those tests now also show that a failed morning sends
   the operator the `…-failed` notice. #64 stays open until someone closes it by hand, since
   dev isn't the default branch. Known gap: `fakes(…)` still returns the plain `Handler` type,
   so `clock(…).calls(op)` fails only at run time; a separate `Fakes` type would catch it.
2. **Watch the digest's first real run in the homelab** (below), and fix what it shows.
3. **Done: the parser fuzz after lane B's grammar change** (eea8493, string effect targets).
   `FUZZTIME=60s FUZZWORKERS=2 task fuzz` ran clean on dev at 386cc1e.

## CI and checks

- **CI on dev was red from fca3a9f to d45535b,** all day, and nobody looked at it: every merge
  was checked with a local `task check`, which can't run the Docker tests on this laptop and
  skips the VS Code window test on macOS. Two causes, both fixed in bbf2505, whose CI run passes:
  - `task check`'s vscode step called `{{.TASK_EXE}}`, which CI's Task leaves empty, so on Linux
    it failed before the VS Code end-to-end test (from fca3a9f).
  - Once `pub` was enforced (25e1998), the exit test's `main` couldn't start `digest.morning`,
    so `TestMorningDigestOnPostgres` timed out waiting for the model. The test's copy of the lab
    now makes `morning` public, and checks the copy before `eye run` starts.
- bbf2505's CI run is the first in which the VS Code end-to-end test checked completion
  (b297dbf, eyelang-3b) and the Postgres exit test ran on today's lab. Both pass.
- **After every push to dev, check CI:** `gh run list --branch dev --workflow ci.yml --limit 3`.
  A local `task check` is not enough: CI runs the Docker tests (`EYE_TEST_DOCKER=1`) and the
  VS Code window test, and this laptop runs neither by default.
- Locally, Docker didn't answer at the end of the day (`docker info` hung), and the disk had
  about 11 GB free. `go clean -cache` frees the most; it filled to ENOSPC once.

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

Runtime and tools:
- `http.get` and `ai.generate` should retry an unreachable service the way contract calls do
  (see the risk above).
- `ai.generate` has no `reasoning_effort` parameter; the TypeScript digest sends "medium".
- Replacing a key's value in a map (`m |> insert(k, f(m[k]))`, counting words) still copies the
  map, so a loop that updates one map is quadratic. Adding a set's element or a map's new key no
  longer is: they share a `shelf` the way lists share a buffer (`interp/value.go`). Slots are
  written once, so a replaced value needs a different structure. String building is most of
  what is left of `plain()`'s 0.47 s.

Checker:
- A command that returns a record can't have `Uncertain` matched. The lab's `createCategory`
  wraps its result in a one-variant type to work around it.
- `consume` isn't inferred from `@on(event(…))`, because a job has no `uses` clause.
- Generic private functions: `double(2.5)` gives two errors where one would do, and calling
  itself at a bigger type isn't detected.
- `@allow(nobody)` is unchecked; it belongs to 6.2 (#54).
- `eye fix` doesn't add the `!` that `publish!` and `start!` now need in jobs; the call sites
  were changed by hand.
- `check/resolve_test.go`'s unresolvable list keeps `nick` and `nickname` on purpose: a
  migration reads the old row by name, which nothing declares, so they don't resolve. `unix` and
  `name` left it today.

Library:
- No hour-and-minute format for a time of day; the digest takes the first five characters of
  its text.
- Fixed-width integer methods (`Int32.from`, `wrappingAdd`, bit operations) are blocked: at run
  time a fixed-width value is a plain Int, so a method can't tell `Int32` from `Byte`. They need
  the typed IR (#28) to record each literal's width or each call's resolved impl. `Int32.from(n)`
  also needs calls on built-in type names.
- Not built: `fs.lines`, reading a file as bytes, fakes for `process`, regular expressions. The
  digest's `plain()` strips HTML without them.

Syntax, open since the parser freeze ended (from the 2026-10-11 lane list, none started):
- Open questions 13 (`eye fmt` and the two forms of `if`), 14 (`if let`), 25 (naming a record's
  type at its literal) and 31 (`Option` parameters with defaults). Each needs a decision first.
- A character literal read as a `Byte`, so list patterns over `Bytes` work. It needs the checker
  and possibly syntax, and the self-hosted lexer (ROADMAP "What's next", item 4) needs it.

Lab:
- `trains.watch` (`testdata/lab/trains/trains.eye`) is still a `while true` loop that sleeps 45 s,
  about 7,700 steps a day. Each wake replays every step so far, so its cost grows with the run's
  age. Until saved frames (7.3, #76) land, give it the restart pattern SPECS §15 describes: end
  the run and start the job again with `watch.start!(t)`, about every hour.

## Next work, in order

This follows ROADMAP's "What's next", with what today changed:

1. Done: #69 (C.5). `eye fmt DIR` writes each public function's `uses` clause by default, as
   the user chose; `--syntax-only` turns it off, and a file formatted alone stays syntax-only.
   SPECS §12 and the decision log ("Writing effects") have the rule. It didn't touch `syntax/`.
2. C.8 (#71), deferred by the user. Approvals are stubbed instead: the lab answers only with
   `eye approve` and `eye decline`, and `approving` sends the operator a message saying how,
   with no buttons. `testdata/programs/jobs/approvals.eye` and
   `TestApprovalWaitsAcrossRestartsAndExpires` show waiting, `within:`, Expired, decline and a
   restart of `eye run` around it. Buttons come back with #71's answers by id.
3. Done: C.11 (#74), module roots. `testdata/lab/deploy/digest` has no symbolic links; its
   `eye.toml` says `modules = ["../.."]`. **For eyelang-af:** from this commit on, the image
   copies the whole lab (`cp -R testdata/lab /lab`) and the program is `/lab/deploy/digest`, not
   `/program`, so the pod's command, `eye runs` and the inbox path change when the pin moves past
   it (testdata/lab/README.md, "The image"). Next: 6.2 (#54), `@port` and `@allow`, now
   unblocked.
4. Phase 6's exit test: 6.6 (#58), Notify generated from its contract, and 6.8 (#60), the
   `agents` module, which needs 6.7 (#59), model `tools:`.
5. 7.1 (#28), the typed IR, and 7.3 (#76), saved frames, both pulled into v0.3.

## Each session's state

What every session that worked today said was left, at the end of the day. Claims are in
`task next`; each owner releases its own with `task claim -- --release WHO` when the user says so.

- **eyelang-20 (phase 5, v0.2.0, merges into dev).** Nothing unmerged. Claim: `dev` (merges in
  the main checkout) and `tools/next`; the next session that merges takes the `dev` claim.
  Open: a loop that replaces values in one map is quadratic (above).
- **eyelang-77 (lanes B to F, this handoff).** All lanes stopped and merged, `track/runtime-wip`
  last. Its worktrees `handoff` and `runtime` are merged and can be removed.
- **eyelang-55 (language server, parser).** Nothing unmerged; `ts-parser`, `list-append`,
  `list-comment`, `lsp-*` and `builtin-decls` are merged and removed. Claim: `internal/lsp`
  except completion, `check/index.go`, `check/resolve_test.go`. Not started, nobody owns them:
  - hover shows a private function's inferred effects;
  - the checker works around a syntax error, skipping only the declaration that has it, so type
    errors, hover and completion keep working while a file is half typed.
  For the next syntax change: golden messages in `testdata/syntax/invalid/diagnostics.txt`
  (`go test ./syntax -run TestSyntaxErrors -update`), conformance against tree-sitter in the
  same process, `tree-sitter-eyelang/src` committed and diffed by `task generated`. `eye` stays
  cgo-free; the tests need a C compiler. Keep the tree-sitter binding in
  `tree-sitter-eyelang/src`: Go's build cache doesn't see C files included from another
  directory.
- **eyelang-42 (docs site).** Merged (`docs-site` at 1b2cb22); a VitePress dev server still runs
  from that worktree on http://localhost:5173. Its claim marks `docs/` as written by hand: tell
  them, or the user, before large changes there. Open: about 20 checker and interpreter bugs the
  tutorial and guide writers hit, most passing `eye check` and failing at run time. eyelang-15
  holds the full list and asked the user whether to file them; lane B may have fixed some. They
  include `Int * Duration`, iterating a view, `?` not wrapping `String` errors, `ai.generate` for
  maps, tuples and `Time`, and a faulted run staying failed.
- **eyelang-3b (editor and tasks).** Nothing open. Claim: `editors/vscode` and the `vscode:*`
  tasks; the rest of `Taskfile.yml` is shared. Open, low priority: CI could cache
  `editors/vscode/node_modules` by `package-lock.json` (about 4 s); the extension's README
  doesn't say whether hover works while a program has type errors.
- **eyelang-17 (completion).** Merged; its worktrees are removed and its claim released.
- **eyelang-af (homelab deploy).** In the stacks-homelab repository; see "The digest in the
  homelab" above.

## How today ran

- Several sessions shared the laptop, and the disk filled once (ENOSPC). Keep to two or three
  lanes, use targeted tests while iterating, and run one full `task check` at a time. Don't run
  `task fuzz` while the disk is tight. When the shared Go cache breaks, a private `GOCACHE` in
  the scratchpad works.
- eyelang-20 merged into dev in the main checkout. It rebased lanes onto one another, ran the full
  check, and fast-forwarded. A lane whose branch was rebased by the merger continues with
  `git rebase --onto dev OLD_TIP`.
- Decided today, and recorded in SPECS.md's decision log:
  - every proposal in [docs/internals/proposals.md](docs/internals/proposals.md);
  - the C back end, with the `zig cc` that `eye` ships;
  - string positions are `Offset`s;
  - effect names (`publish Bus.pinged`, `secret "name"`);
  - a step's result must be saveable;
  - private functions are generic;
  - `with` binds a fake handler;
  - the formatter writes public functions' `uses` clauses (#69).
- Closed today: #62 (`extern type` replaces `Handle`) and #68 (`NonZero` divisors). #72 was
  closed by its commit.
