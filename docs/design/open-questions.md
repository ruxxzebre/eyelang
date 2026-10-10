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
4. **Job code upgrades:** when a new version changes what is live at a checkpoint, do paused runs
   finish on the old version, or must the change come with a migration?
5. **Packages:** `eye.toml` and an `eye.lock` that carries hashes (so Nix needs no placeholder
   hash); registry or git only? `eye.toml` already marks a program's root ([section 13](/reference/language/programs-and-concurrency)); what it
   holds beyond that is open.
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
     keeps every step: four or more per loop, so 7,700 or more a day per person, each wake
     replaying all of them, so its cost
     grows with the square of its age. `eye run` warns past 1,000 steps and `eye runs` flags such
     runs. The fix is the model [section 15](/reference/language/jobs-and-waiting) describes, saving the live variables at each `!`
     instead of the history, which compiled jobs will do; until then, a loop can end by starting
     its job again (`watch.start(t)`) so that each run stays short. Should the language say so,
     or offer `continue`-as-new for jobs?
   - One process per journal (a file lock); several runners need the Postgres store.
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
15. **Typed choices.** A question's buttons are strings (`choice: "boarded"`), and the answer is
    matched as a string. Could a variant type give the choices, with labels from `##` comments, and
    the answer come back as that type?
16. **What a fake received.** A test can't see which calls a fake answered, so the lab's digest
    records the keys it sent with in an entity to make sending testable. Should a test be able to
    ask `fakes(notify).sent(…)`, or fail when a recording has exchanges no call used?
17. **Where approvals go.** Tests answer `approval!` with `approve(…)` and `decline(…)`. How does a
    deployment say where requests go and who may answer (the lab would use Notify's buttons)?
18. **Contracts shared by programs.** The lab is one program so that its modules share the Notify
    contract; separate programs would each need a copy. This belongs with packages (question 5).
19. **Which actions are effects?** Calling a service, waiting, touching entities, the clock and
    standard output are. Are logging, publishing an event and starting a job effects too, to be
    stated with `uses`? (`publish T` is in the table of [section 12](/reference/language/effects-and-signatures); the checker doesn't infer it yet.)
20. **Collection functions' types:** decided on 2026-10-11, for v0.1: one function per operation,
    with a rule for each kind of collection it takes (lists, sets, maps, ranges, `Option`s, and
    strings where it makes sense), and a requirement where one is needed (`sort` needs `Ord`,
    `toSet` `Hash`, `sum` `Add`). A `Collection` trait would need traits over type constructors,
    which v0.1 doesn't have; reopen this if user-defined collections want the same pipelines.
