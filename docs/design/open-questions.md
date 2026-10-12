---
title: "Open questions"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Open questions

1. **Time conditions in views**, such as "idle for 20 minutes". Today that is `enters(view)` plus
   `sleep!`. Should views support `for 20 min` directly?
2. **Trait coherence:** decided on 2026-10-11: no. An impl lives in the module of the trait or of
   the type ([section 11](/reference/language/traits)).
3. **Exposing entities and jobs to other services:** a generated contract per entity, or explicit
   contracts only?
4. **Job code upgrades:** decided on 2026-10-11: detect, and refuse. A paused run resumes only at
   a resume point that still exists with the same live variables; `eye run` refuses to start
   otherwise, and until runs have frames, replay faults a run whose recorded call differs from
   the one it replays ([section 15](/reference/language/jobs-and-waiting)). Finishing old runs on old code is rejected, since entities
   migrate forward. A `migrate` for a job's resume points waits for a program that needs it.
5. **Packages:** `eye.toml` and an `eye.lock` that carries hashes (so Nix needs no placeholder
   hash); registry or git only? `eye.toml` marks a program's root and may list `modules`, more
   directories to find imports in ([section 13](/reference/language/programs-and-concurrency)); what it holds beyond that is open.
6. **REPL behaviour for jobs:** decided on 2026-10-10: `eye repl` runs jobs against the in-memory
   store on a clock that moves only when `:advance 20 min` says, as a test's does; `:runs` shows the
   runs as `eye runs` does. The session keeps its entities, runs and bindings while declarations
   change.
7. **Metaprogramming after v1.0:** if real code keeps repeating the same patterns, add attribute
   macros (compile-time Eyelang functions that inspect a declaration's types and generate code).
   The built-in constructs could then be re-expressed with them, and grammar and editor support
   would stay unchanged because macros would only appear as attributes.
8. **Multi-line strings:** decided on 2026-10-10: `"""`, with the closing line's indentation as
   the margin ([section 4](/reference/language/lexical-structure)).
9. **Durability without a database:** decided on 2026-10-10: the journal. Each commit (entity
   writes, runs, events, schedule firings, event keys seen, commands whose reply was lost) is one
   line of JSON appended to `.eye/journal` and synced before it counts. A commit that can't be
   written, on a full disk or otherwise, changes nothing: the journal is cut back, memory isn't
   touched, and the run stays at its last checkpoint and is tried again later. The journal is
   rewritten as one snapshot when it is opened and when it grows past four times its live data,
   and a 4 MiB reserve file beside it is given up when a write fails, so that compaction can still
   finish. Events no unfinished run can still see are dropped, an event's key is forgotten after
   30 days, and a run that finished more than 7 days ago keeps a summary (job, arguments, outcome,
   times, step count) instead of its steps; waiting runs and unsettled lost commands are never
   dropped. Still open:
   - The retention times are fixed; should a program set them, and should `eye runs` show what
     was summarised?
   - A run that never ends, such as the lab's reminders watcher (`while true { …; sleep!(45 s) }`),
     kept every step, four or more per loop, each wake replaying all of them. Decided and done on
     2026-10-11: runs save their variables instead of their history, at the statements of the
     job's body in the interpreter (ROADMAP task 7.3) and at each `!` in compiled code, and the
     language gets no `continue`-as-new. `eye run` warns about a run that has taken 1,000 steps
     since its checkpoint, as one whose loop is inside a function the job calls once.
   - One process per journal (a file lock), and one per Postgres database and schema (an
     advisory lock). Several runners on one database is question 28.
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
15. **Typed choices:** decided on 2026-10-11: a type whose variants have no fields gets
    `T.variants()` and `T.parse(s)`, derived ([section 11](/reference/language/traits)), so the answer is matched as a variant.
    Contracts stay untyped over the choice, since Notify receives strings on the wire. Still open:
    should a choice's label be its `##` comment, or its `Show`, with an impl where the text differs
    from the variant's name? `##` comments are documentation, and using them as text for people
    mixes two jobs; an impl of `Show` is the existing way to choose how a value is written. Until
    this is decided, a label is the variant's `Show`.
16. **What a fake received:** decided on 2026-10-11: `fakes(…)` is a value that a test names and
    applies with `with`, and `sent.calls(notify.send)` lists the calls the fakes answered, as
    records of their arguments with defaults filled in and the key ([section 17](/reference/language/contracts-talking-to-other-services)). The lab's digest
    no longer keeps the keys it sent with in an entity. Whether a recording's unused exchanges
    should fail a test is still open: one recording serves several tests.
17. **Where approvals go:** decided on 2026-10-11: through the program. A run that waits at
    `approval!` publishes `approvals.requested`; the program routes it with a job of its own (the
    lab's `approving` module asks the operator on Notify) and answers with `approvals.answered`,
    which `eye approve`, `eye decline` and tests also publish ([section 15](/reference/language/jobs-and-waiting)). And decided on
    2026-10-11: an answer names its request's `id`, and an answer whose `by` isn't the request's
    `from` is refused; text matches only from the command line and in tests, and is refused when
    it matches more than one waiting approval; an `Approval` has `what` and `by` fields. Built on
    2026-10-12, where tests and the command line also refuse text that matches no waiting
    approval, and answer as the person asked when they name no one. Who may publish an answer
    at all is question 35.
18. **Contracts shared by programs.** The lab is one program so that its modules share the Notify
    contract; separate programs would each need a copy. This belongs with packages (question 5).
19. **Which actions are effects?** Decided on 2026-10-11: every effect in [section 12](/reference/language/effects-and-signatures)'s table is
    inferred, `publish C.event`, `consume C.event`, `secret "name"` and `ffi` included, and `!`
    marks every recorded step in a job, so publishing is `publish!` and starting a child run
    `start!` ([section 12](/reference/language/effects-and-signatures)). Logging is not an effect.
20. **Collection functions' types:** decided on 2026-10-11, for v0.1: one function per operation,
    with a rule for each kind of collection it takes (lists, sets, maps, ranges, `Option`s, and
    strings where it makes sense), and a requirement where one is needed (`sort` needs `Ord`,
    `toSet` `Hash`, `sum` `Add`). A `Collection` trait would need traits over type constructors,
    which v0.1 doesn't have; reopen this if user-defined collections want the same pipelines.
21. **`Handle(name)` and `extern type`:** decided on 2026-10-11: `Handle` goes, and `extern type`
    is the one way to name what foreign code owns. A program's `extern type` has no `Save`,
    `Show`, `Eq`, `Hash` or `Schema` ([section 18](/reference/language/calling-c-and-go)).
22. **Printing in jobs:** decided on 2026-10-11: output stays outside the `!` rule. `print` keeps
    `stdio` for signatures but is not a step, logging is not an effect, and the interpreter keeps
    a run quiet while it replays the steps since its checkpoint ([section 12](/reference/language/effects-and-signatures)).
23. **Where `!` goes on a helper.** Writing `fn stamp() = now()` and calling `stamp!()` makes the
    whole helper one recorded step; a helper that itself has `!` steps is called with `!` too, but
    then its steps are recorded one by one. The same mark means "record this call" in one case and
    "this call contains steps" in the other. Is one mark for both right, since either way the
    caller sees a point where the run may stop and resume, or should the difference show?
24. **`dyn Store`, and traits that name their own type.** The decision log's own example of
    `dyn` is choosing a store at run time, but the `Store` trait is `Store(s)`, whose `commit`
    returns `s`, the store's own type, so a `dyn Store` would need to say which store it is. A
    trait has no name for "the type that implements me" (`Self`), and threads it through a
    parameter instead, as `Add(rhs, out)` does. Should traits get `Self`, with traits whose
    methods return `Self` kept out of `dyn` (as Rust does), or should a store's `commit` change
    the store in place (`mut self`) so the trait needs no type of its own?
25. **Naming a record's type at its literal.** Records are nominal and a literal takes the type
    its context expects. Where the context is a `dyn Trait`, or a list of them, nothing says
    which type a literal is, so each implementation needs a constructor (`Square.new(2)`) or a
    `let s: Square = { side: 2 }` first. Should a literal be able to name its type, as
    `Square { side: 2 }` does elsewhere, or is a constructor the one obvious way?
26. **What `@memory` measures.** A compiled job would count its process's memory. The reference
    interpreter runs one run at a time and counts what it keeps for a run: its saved state, with
    every step's result. So "lower priorities pause at their next checkpoint" happens only in that
    a run waiting for memory isn't overtaken; nothing is paused mid-run. Is saved state the right
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
34. **An online playground.** A page where you write Eyelang and check, test and run it, with the
    editor support `eye lsp` gives (errors at their spans, completion, hover, formatting), and
    examples to start from. The checker, interpreter, language server and parser are Go without
    cgo and already build for WebAssembly (`GOOS=js GOARCH=wasm`), so the page could run them
    with no server, behind an editor such as CodeMirror. What it would take: the checker reads
    a program's files from `Options.Overlay` but still lists its directory on disk, so a
    program held only in memory must be accepted; the language server needs an entry point that
    takes text and a position instead of a stdio connection; `eye run`'s jobs need a clock that
    yields to the page; and model calls need recorded fakes or a key the visitor brings.
    Contracts are the case that shows the language best: two programs side by side, one that
    serves `impl Notify` on its `@port` and one that calls `notify.send!`, with `@allow` and
    port errors at compile time and the calls visible as they cross. In the page, the caller's
    `@http` calls could go to the other program's handler in memory; from the command line, two
    `eye run`s on real ports, which works since 2026-10-12, when `eye run` started serving a
    program's `impl Contract` on its port ([section 17](/reference/language/contracts-talking-to-other-services)). Is the playground worth building before
    v1.0, and should it share its examples with the docs site and the tour?
35. **Who may publish an event.** `@allow` names the modules that may call a contract's
    commands and queries, but any module may publish any of its events. For
    `approvals.answered` that matters: the runtime checks that `by` is the request's `from`
    ([section 15](/reference/language/jobs-and-waiting)), but a module that writes `by: r.from` itself grants consent with no person
    involved. One way: `@allow(approving)` on an event, checked like a command's, so only the
    module that hears people may answer, and `eye approve` and tests stay outside it as the
    runtime's own. The standard `approvals` module can't name the program's modules, so the
    program would need a place to say it, such as `eye.toml` or an attribute on the routing job
    (`@answers(approvals)`). Is a guard on publishing worth a new rule, and where does it go?
