---
title: "Open questions"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Open questions

1. **Time conditions in views**, such as "idle for 20 minutes". Today that is `enters(view)` plus
   `sleep!`. Should views support `for 20 min` directly?
2. **Trait coherence.** **Decided** (see the decision log, row "Shared behaviour"): an impl lives
   in the module of the trait or of the type ([section 11](/reference/language/traits)).
3. **Exposing entities and jobs to other services:** a generated contract per entity, or explicit
   contracts only?
4. **Job code upgrades.** **Decided** (see the decision log, row "Job code changes"): detect, and
   refuse ([section 15](/reference/language/jobs-and-waiting)). Still open: a `migrate` for a job's resume points, once a program needs
   one.
5. **Packages:** `eye.toml` and an `eye.lock` that carries hashes (so Nix needs no placeholder
   hash); registry or git only? `eye.toml` marks a program's root and may list `modules`, more
   directories to find imports in ([section 13](/reference/language/programs-and-concurrency)); what it holds beyond that is open.
6. **REPL behaviour for jobs.** **Decided** ([section 13](/reference/language/programs-and-concurrency), `eye repl`): jobs run against the
   in-memory store on a clock that moves only with `:advance`, as a test's does.
7. **Metaprogramming after v1.0:** if real code keeps repeating the same patterns, add attribute
   macros (compile-time Eyelang functions that inspect a declaration's types and generate code).
   The built-in constructs could then be re-expressed with them, and grammar and editor support
   would stay unchanged because macros would only appear as attributes.
8. **Multi-line strings.** **Decided** (see the decision log, row "Strings"): `"""`, with the
   closing line's indentation as the margin ([section 4](/reference/language/lexical-structure)).
9. **Durability without a database.** **Decided** (see the decision log, rows "Storage" and "Long
   runs"): the journal. Each commit (entity
   writes, runs, events, schedule firings, event keys seen, commands whose reply was lost) is one
   line of JSON appended to `.eye/journal`, and synced, in a group with the commits before it,
   before anything outside the runner can see it ([section 15](/reference/language/jobs-and-waiting)). A commit that can't be
   written, on a full disk or otherwise, changes nothing: the journal is cut back, memory isn't
   touched, and the run stays at its last checkpoint and is tried again later. The journal is
   rewritten as one snapshot when it is opened and when it grows past four times its live data,
   and a 4 MiB reserve file beside it is given up when a write fails, so that compaction can still
   finish. Events no unfinished run can still see are dropped, an event's key is forgotten after
   30 days, and a run that finished more than 7 days ago keeps a summary (job, arguments, outcome,
   times, step count) instead of its steps; waiting runs and unsettled lost commands are never
   dropped. Runs save their variables, not their history (row "Long runs"), and `eye run` warns
   about a run that has taken 1,000 steps since its checkpoint. One process writes a journal at a
   time; several runners on one database is question 28. Still open: the retention times are
   fixed; should a program set them, and should `eye runs` show what was summarised?
10. **Streaming model output:** `ai.generate!` returns a whole, checked value. Interfaces that show
    a reply as it arrives want partial results; should `ai.stream` yield an `Iter` of partial
    values, checked as far as they go?
11. **Prompts as text:** prompts are multi-line strings now (question 8). Should long prompts also
    live in template files next to the code, loaded and checked when the program is compiled?
12. **Request-scoped values,** such as a trace ID or the current user. Go passes them in a
    `context.Context`. Eyelang has no such parameter: cancellation and deadlines come from
    structured concurrency, and capabilities from effects. Today such values are passed as
    arguments. Should `with` provide a value to the rest of a block, as it provides fakes and clocks
    in tests, with the dependency shown in signatures as an effect?
13. **`eye fmt` and the two forms of `if`.** Should the formatter rewrite a one-line
    `if c { a } else { b }` to `if c then a else b`, so that each `if` has one canonical form?
14. **`if let`.** Writing the lab, `if let Some(a) = answer then return Err(Taken(a))` came to hand
    twice, where today a `match` with a `None => ()` arm does the same in four lines. `let … else`
    covers the opposite case. Is a third form worth it, or does `match` stay the one way?
15. **Typed choices.** **Decided** (see the decision log, row "Typed choices"): `T.variants()`
    and `T.parse(s)`, derived ([section 11](/reference/language/traits)). Still open:
    should a choice's label be its `##` comment, or its `Show`, with an impl where the text differs
    from the variant's name? `##` comments are documentation, and using them as text for people
    mixes two jobs; an impl of `Show` is the existing way to choose how a value is written. Until
    this is decided, a label is the variant's `Show`.
16. **What a fake received.** **Decided** (see the decision log, row "What a fake received"):
    `sent.calls(notify.send)` ([section 17](/reference/language/contracts-talking-to-other-services)). Still open: should a recording's unused exchanges
    fail a test, when one recording serves several tests?
17. **Where approvals go.** **Decided** (see the decision log, row "Answering approvals"), and
    built: through the program, which routes `approvals.requested` and answers with
    `approvals.answered`, naming the request's `id`, from the person in `from:` ([section 15](/reference/language/jobs-and-waiting)). Who
    may publish an answer at all is question 35.
18. **Contracts shared by programs.** The lab is one program so that its modules share the Notify
    contract; separate programs would each need a copy. This belongs with packages (question 5).
19. **Which actions are effects?** **Decided** (see the decision log, row "Recorded steps"):
    every effect in [section 12](/reference/language/effects-and-signatures)'s table is inferred, and `!` marks every recorded step in a job.
20. **Collection functions' types.** **Decided** for v0.1 ([section 18](/reference/language/calling-c-and-go)): one function per
    operation, with a rule for each kind of collection it takes and a requirement where one is
    needed (`sort` needs `Ord`). A `Collection` trait would need traits over type constructors;
    reopen this if user-defined collections want the same pipelines. The built-ins work as
    methods, `xs.first()`, as every function whose first parameter takes the value does
    ([section 8](/reference/language/methods-and-generics), and the decision log, row "Functions as methods").
21. **`Handle(name)` and `extern type`.** **Decided** (see the decision log, row "Foreign
    values"): `extern type` is the one way to name what foreign code owns ([section 18](/reference/language/calling-c-and-go)).
22. **Printing in jobs.** **Decided** (see the decision log, row "Recorded steps"): output is not
    a step, and logging is not an effect ([section 12](/reference/language/effects-and-signatures)).
23. **Where `!` goes on a helper.** Writing `fn stamp() = now()` and calling `stamp!()` makes the
    whole helper one recorded step; a helper that itself has `!` steps is called with `!` too, but
    then its steps are recorded one by one. The same mark means "record this call" in one case and
    "this call contains steps" in the other. Is one mark for both right, since either way the
    caller sees a point where the run may stop and resume, or should the difference show?
24. **`dyn Store`, and traits that name their own type.** Choosing a store at run time is an
    obvious use of `dyn`, but the `Store` trait is `Store(s)`, whose `commit`
    returns `s`, the store's own type, so a `dyn Store` would need to say which store it is. A
    trait has no name for "the type that implements me" (`Self`), and threads it through a
    parameter instead, as `Add(rhs, out)` does. Should traits get `Self`, with traits whose
    methods return `Self` kept out of `dyn` (as Rust does), or should a store's `commit` change
    the store in place (`mut self`) so the trait needs no type of its own?
25. **Naming a record's type at its literal.** **Decided** (2026-10-12): no syntax at the
    literal; a lambda's record literal takes its type from where the result goes, as other
    literals do, so `users |> map(fn u: { name: u.name })` builds the record type the result is
    used as ([section 5](/reference/language/values-and-types), and the decision log, row "Record literals in lambdas"). Still open:
    where the context is a `dyn Trait`, nothing
    says which type a literal is, so each implementation needs a constructor (`Square.new(2)`)
    or a `let s: Square = { side: 2 }` first. Is that enough?
26. **What `@memory` measures.** A compiled job would count its process's memory. The reference
    interpreter runs one run at a time and counts what it keeps for a run: its saved state, with
    every step's result. So `@priority` acts only in that a run waiting for memory isn't
    overtaken by a lower one; nothing is paused mid-run. Is saved state the right
    measure until v0.4, and should `eye runs` show each run's against its `@memory`?
27. **One flag for the store.** `eye run` takes `--journal file`, `--store postgres://…` and
    `--memory` (keep nothing), and now `--memory-limit`, which reads like the third. Should the
    store be one flag, `--store file:… | postgres://… | memory`, and should `eye.toml` name a
    program's store, including the Postgres schema that today goes in the address as
    `?search_path=`?
28. **Several runners on one database.** The Postgres store allows one `eye run` per database
    and schema, held by an advisory lock. Running more needs runners to claim runs (leases that
    expire), and the in-memory copy each keeps would no longer be the whole truth. Is that v0.2's,
    or later? And when a commit whose reply was lost turns out to have arrived, `eye run` stops so
    a restart reads the database again; should it reload in place instead?
29. **The shape of task groups.** `taskGroup(fn group { … })` with `group.spawn(fn { … })` needs
    no syntax, but a `group { … }` block would read better. A group returns its first function's
    result; should it gather its tasks' results instead? `channel(capacity:)` returns a tuple to
    destructure; would one `Channel(a)` with `put`, `take` and `close` be simpler, at the cost of
    handing both ends to everyone? A group whose tasks all wait on channels nobody can reach fails
    (a forgotten `close()`); should a `Sender` close itself when the task that owns it ends?
30. **Model prices and routing.** Dollar budgets need prices: Anthropic's are built in, others
    come from `AI_PRICES`, and a dollar budget on a model with no known price stops the run. Should
    prices live in `eye.toml`? `AI_URL`, when set, sends every model through the gateway; is an
    explicit routing table clearer? A function's `@budget` is per call; should it pool across the
    run? And the prompt is estimated at about 3 bytes a token; should the runtime ask the
    provider's token counter instead, at the cost of a call?
31. **`Option` parameters with defaults.** `ai.generate(…, provider: Some(ai.Anthropic))` reads
    clumsily: the parameter is `Option(Provider) = None` so that leaving it out means "from the
    model's name". Should a call be able to pass a bare value to such a parameter, or should a
    default be able to say "chosen later" without `Option`?
32. **Time-travel debugging.** A job's run is deterministic between its `!` steps, which record
    everything that could differ (the clock, the network, randomness, entity reads, model calls),
    and tests run on a test clock against recorded fakes. Since checkpoints ([section 15](/reference/language/jobs-and-waiting)) save a
    run's variables at its statements, any moment of a run can be reached again: restore the
    nearest checkpoint before it and replay the recorded steps up to it, calling nothing outside.
    Stepping back is restoring an earlier checkpoint and replaying forward to the step before.
    That asks for: a run that keeps its history on request (`eye run --history`), since a
    checkpoint drops the steps before it and the journal compacts; `eye debug RUN` and
    `eye test --debug` that move a run or a failing test forward and back by statement and step,
    showing its typed variables (7.1) and the entities as they were; and a Debug Adapter Protocol
    server, whose stepBack and reverseContinue give editors the same. Which first, and how much
    history may a run keep? A failing test is the cheapest start, since tests are deterministic
    already.
33. **Effects of a function that is only stored.** A lambda or a named function counts its
    effects where it is written, even when it goes into data whose function type states them, so
    a function that only builds a record of functions needs their `uses` (`testdata/ir/effects`).
    Since a function kept in data is held to its type's `uses` ([section 12](/reference/language/effects-and-signatures)), a value that only
    ever meets such limits could stop counting where it is written, and `eye fmt` would stop
    writing those effects into signatures. Is the clause worth keeping as a reader's hint, or
    should it go?
34. **An online playground.** **Decided, and built** on 2026-10-12: the docs site's playground
    (`docs/playground.md`) is a page where you write Eyelang and check, test, run and format it,
    with the editor support `eye lsp` gives (errors at their spans, hover, completion, go to
    definition, formatting) and highlighting from the parser. Nothing runs on a server: the
    checker, interpreter, formatter and language server are built for WebAssembly (`cmd/eye-wasm`,
    `GOOS=js GOARCH=wasm`) and run in a web worker, behind CodeMirror. A program held only in
    memory is accepted: the checker takes a directory that exists only in `Options.Overlay`, the
    language server answers through `lsp.Workspace` with no connection, and the fakes read
    recordings from memory. `eye run`'s jobs run on a virtual clock (`interp.RunOptions.Virtual`):
    once `main` has run, sleeping runs wake and schedules fire at once, in time order, up to a day
    of clock time or a thousand wakes, and nothing listens on a port. A run that doesn't end is
    stopped after ten seconds, or with Stop. Its examples are the tutorial's and the guide's
    programs, which the Go tests and the playground's browser test run there, and each of those
    pages links to its program in the playground. A program can be shared as a link that holds its
    files. Contracts work between two programs in the page: a directory with an `eye.toml` of its
    own is a second program, and Run starts each such program that serves a contract as a service
    in memory (`interp.StartService`), with no port, then sends the root program's `@http` calls to
    its handlers (`interp.RunOptions.Services`), as the HTTP requests two `eye run`s would
    exchange; each request shows with its status as it crosses, the secrets the contracts' `@auth`
    names are made up for both sides, a call moves the service's clock to the caller's, and a retry
    moves the clock instead of waiting (`docs/examples/shop-and-bank`). Still open: model calls
    with a key the visitor brings, which today only replay recordings.
35. **Who may publish an event.** `@allow` names the modules that may call a contract's
    commands and queries, but any module may publish any of its events. For
    `approvals.answered` that matters: the runtime checks that `by` is the request's `from`
    ([section 15](/reference/language/jobs-and-waiting)), but a module that writes `by: r.from` itself grants consent with no person
    involved. One way: `@allow(approving)` on an event, checked like a command's, so only the
    module that hears people may answer, and `eye approve` and tests stay outside it as the
    runtime's own. The standard `approvals` module can't name the program's modules, so the
    program would need a place to say it, such as `eye.toml` or an attribute on the routing job
    (`@answers(approvals)`). Is a guard on publishing worth a new rule, and where does it go?
36. **`@destructive` means two things.** On a command it adds an `Approval` parameter, which the
    caller gets from `approval!` ([section 17](/reference/language/contracts-talking-to-other-services)); on a function given to a model as a tool, it
    pauses the job until someone approves that one call ([section 19](/reference/language/ai-models-and-agents)), though nothing says who is
    asked, as `approval!`'s `from:` does. Should the tool form take the same `Approval`, or have a
    name of its own and say whom it asks?
37. **One shape for "time ran out".** `poll!` gives `false`, `next!` gives `None`, and
    `approval!` gives `Expired` when their time runs out ([section 15](/reference/language/jobs-and-waiting)). Should every wait with a
    limit end the same way, such as a variant `TimedOut` or an `Option`?
38. **Slices and memory.** `xs[2..5]` and `bytes[i..]` share storage with what they come from
    ([section 5](/reference/language/values-and-types)), so a 10-byte slice keeps a 100 MB `Bytes` alive, a cost nothing shows. What is a
    slice's type, the type it was cut from or one of its own, and should keeping a small slice of
    a large value copy it? Compiled by `eye build`, a list's slice shares its buffer since 7.13,
    but a slice of `Bytes` or a string still copies what it covers until they are held as views
    (docs/internals/compilation.md, [section 2](/design/decision-log)), so
    matching `src[pos..]` once a token makes a lexer quadratic; the lexer of S.1
    (`testdata/programs/lexer`) matches a few bytes at a time, `src[pos..pos + 3]`, for that
    reason.
39. **A module with two contracts.** A module that declares one contract offers its operations
    and events by name, `notify.send` ([section 17](/reference/language/contracts-talking-to-other-services)). With two contracts, two operations or events
    could share a name. Should a module declare at most one contract, or should calls name the
    contract, `notify.Notify.send`?
40. **What `main` repeats.** `main` runs on every start of `eye run`, and the key rule stops it
    starting a job twice ([section 13](/reference/language/programs-and-concurrency)), but its entity calls and publishes happen again on every
    start: `pub fn main() { Ledger("x").credit(10) }` credits on each restart. Should `main` be
    held to keyed work only, or should a program's setup move to something that runs once?
41. **Runs pinned to the code they started on.** Any change to a job or a waiting function where
    runs are paused stops them: `eye run` refuses to start, or a replayed step faults (`the code
    of … changed while this run was paused`), and the way out is to keep the old job under its
    name or end the runs ([section 15](/reference/language/jobs-and-waiting)). Making the lab's `web.page` read `env.get!(…)` was such a
    change: one more step, so a digest run stopped while fetching pages faults after the deploy.
    Temporal's worker versioning and Restate's deployments instead finish a run on the version
    it started on. Should the runner keep each version of a program, by the hash of its code,
    while runs paused in it remain, and start new runs on the newest? A job that never ends, such
    as the train watcher, would then need a safe point to move on: the top of its loop, where it
    resumes from its variables if the statement still reads the same, or ending and starting
    itself again with its state. What does `eye runs` show of the versions, and when is an old one
    dropped?
42. **Callbacks across a `!`.** **Decided** (see the decision log, row "Functions across a
    `!`"): a function is saved as its code and what it captured, when the checker sees both, so
    a waiting helper such as `retry` can call a callback after a `!` ([section 15](/reference/language/jobs-and-waiting)).
43. **Byte-level code.** **Decided** (see the decision log, rows "Element assignment", "Hex and
    binary literals", "String patterns over bytes", "Bytes that aren't text" and "A variant's
    number"): Eyelang's lexer in Eyelang (S.1, `testdata/programs/lexer`) asked for five things,
    and each is now a rule: `xs[i] = v` on a `var` or `mut` list and `m[key] = v` on a map
    ([section 7](/reference/language/mutation)), `0xC2` and `0b1010` ([section 4](/reference/language/lexical-structure)), a string literal as a pattern over `Bytes`
    ([section 9](/reference/language/pattern-matching)), `fs.readBytes` and `toBytes` ([section 5](/reference/language/values-and-types)), and `v.index()` ([section 11](/reference/language/traits)).
44. **Patterns checked when the program is compiled.** `regex.parse` gives a `Result` even for a
    pattern written in the program, so a pattern the program knows is right takes a `match` with
    `unreachable`, and a mistake in it shows only when the code runs ([section 5](/reference/language/values-and-types)). A zone's name
    is already checked when the program is compiled, where a `Zone` is expected; a string
    literal where a `Regex` is expected could be too: `let tag: Regex = "<[^>]*>"`. Separately,
    `\` and `{` are escapes in a string, so `\d{4}` is written `"\\d\{4}"`; a raw string would
    let a pattern read as it is written everywhere else. Which of these, if either, is worth it?
45. **Tests that reach the network.** **Decided** (see the decision log, row "Tests don't reach
    the network"): a call to another service in a test, outside `--live` and without
    `fakes(…)` for its module, fails the test with an error that names what to add.
46. **A permission model for effects.** Tests now deny the network unless `--live` names a
    module, and `uses` already says what each public function may touch. Should the runner grant
    effects the same way, so that `eye run` or a deployment lists the services, files and
    processes a program may reach (`--allow net Notify, fs "/data"`), checked against the
    effects the program's signatures declare? Should a test be able to ask for one live module
    itself, and how would that interact with `@allow` and `eye gen policy`?
47. **How fast a million lines compile.** The owner's targets, set on 2026-10-13: `eye build`
    compiles a program of a million lines in at most a minute; in 10 seconds in the long term;
    in 1 second at the furthest. Measured that day on a 10-core Mac (`benchmarks/README.md`):
    checking a million lines takes 3.8 s and writing their C 29 s, 11.6 million lines of C in
    one file, and the optimised C compile then dominates and grows faster than the program:
    10,000 lines build in 18 s, 100,000 in 4 minutes. Unoptimised (`-g`), 100,000 lines build
    in 14 s. Which ways get there: C split into a unit per module, compiled in parallel and
    cached by its hash, so a module that didn't change isn't compiled again; less C per line of
    Eyelang (about 12 now); optimising only where it pays; a back end of our own for debug
    builds, with C for release builds, as Go and Zig do; a checker that runs in parallel and
    holds less (a million lines take 2.7 GB to check)? And does the 1-second target mean a
    build from nothing, or a build after an edit, which caching brings within reach sooner?
