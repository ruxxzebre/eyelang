---
title: "Goals"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Goals

**Eyelang lets you write a program that runs for days (waits for people, keeps state, calls
services that fail) as plainly as a script, and the compiler catches the mistakes such programs
usually ship with.** Two lists follow from that. The **promises** say what a user gets; the README
and the docs site are written from them. The **principles** say how we decide; every change to the
language is weighed against them, and [CLAUDE.md](https://github.com/ruxxzebre/eyelang/blob/dev/CLAUDE.md) points here instead of keeping a copy.

**Promises.** Each is a rule of the language, and its section says how it holds.

1. **Waiting is one line.** `sleep!(3 days)`, `next!(notify.answered, within: 6 h)` and
   `@on(daily(07:40, in: "Europe/Dublin"))` replace the queue, the cron and the `next_run_at`
   column. A waiting run is a record on disk, not a thread: the journal and the Postgres store keep
   only a handle of about 140 bytes in memory, and the run survives restarts
   ([section 15](/reference/language/jobs-and-waiting)).
2. **Replays can't go wrong.** Every durable step in a job is marked `!`, and a resumed run gets
   each step's recorded result instead of calling again. Reading stored state in a job outside a
   step, or keeping a value that can't be saved across a `!`, is a compile error; Temporal finds
   the same mistake at replay time ([sections 12](/reference/language/effects-and-signatures) and
   [15](/reference/language/jobs-and-waiting)). A change to a job's code where runs are paused is refused when
   `eye run` starts, or faults the run when it replays (open question 41).
3. **State has one owner.** An entity's data changes only through its own methods, and each call
   tree is one transaction that commits whole or not at all, with no locks to write
   ([section 14](/reference/language/entities)).
4. **Outside calls are typed and settle once.** A contract types every call to another service,
   and a model call names the Eyelang type its reply must have
   ([section 19](/reference/language/ai-models-and-agents)). A command sent again with the same key doesn't do
   its work twice, and a contract decides whether its commands need one (`@key(required)`). A
   lost reply is `Uncertain`, and where the contract declares `@reconcile`, `.reconcile!()` asks
   the service what happened instead of guessing. The runtime sends again only what is
   safe to repeat, such as a request that never reached the service ([section 17](/reference/language/contracts-talking-to-other-services)).
5. **Days of behaviour test in milliseconds.** A test's clock moves only when it calls
   `advance(3 days)`, its store is in memory, `random` has a fixed seed, and `with fakes(…)`
   replays recorded replies ([section 20](/reference/language/time-secrets-tests)). The approvals tutorial's
   tests, which cover days of waiting, run in about 5 ms. A test without `fakes(…)` can still
   reach the real network (open question 45).

Under the promises is a small, strict core language: no null and no exceptions (`Result` instead),
values instead of shared objects, exhaustive `match`, effects in public signatures with `uses`,
and `mut` marked at the call.

**Principles.** When a choice comes up, weigh it against these, roughly in this order.

1. **Good programs come out.** A feature earns its place by the programs written with it: correct,
   fast, lean on memory, well organised, easy to change, and a pleasure to read, type and review.
   Prefer the design that makes the obvious program the correct one. If the easy way to write
   something is the wrong way, the language is wrong. How the compiler feels to work on comes
   second.
2. **Values, not shared objects.** Data is immutable, mutation is local and visible (`mut`), and
   nothing is aliased. State that several parts of a program share lives in entities (promise 3).
   This is why parallel code has no data races, and why compiled code needs no garbage collector.
3. **Performance by construction.** Native code, reference counting with in-place updates when a
   value is unique, no tracing GC and no GC pauses, and waiting that costs no memory (promise 1).
   Don't add a feature whose cost is hidden or unbounded. Today `eye build` compiles the core
   language to a native binary; entities, jobs, views and contracts run in the interpreter
   (`eye run`), written in Go, until compiled programs are hosted by the runner (ROADMAP 7.6).
4. **Concise, without being cryptic.** Little ceremony, inference inside bodies, and one obvious
   way to do each thing. A line of Eyelang says what it does, and nothing extra. Prefer words to
   symbols unless the symbol is already universal.
5. **Organisation is enforced, not suggested.** State has one owner, effects appear in public
   signatures, matches are exhaustive, and the formatter has no options. Structure that matters is
   checked by the compiler, not left to convention.
6. **Developer experience is part of the language.** Error messages point at exact spans and say
   what to do. Tools are fast. Tests are deterministic. Editor support, formatting and docs ship
   with each feature, not after it.
7. **Strict, but never clumsy.** Every rule must pay for itself in ordinary code. When a rule makes
   normal code awkward, as a reserved `start` did for every lexer, change the rule.
8. **Visible where it matters.** Costs, effects and risks show at the place they happen: `!` for a
   durable step, `mut` for a change, `uses` for effects, `dyn` for a run-time lookup, `unreachable`
   for a stop, and budgets for what a model may spend. Everything a reviewer needs is in the diff:
   a change to a public signature's types or effects, or to where a job can pause, shows there.
   Everything else stays quiet.
9. **A general language first.** Values with no shared mutation, variants with exhaustive
   matching, traits and generics, inferred types, structured concurrency, and no null or
   exceptions. Entities, jobs, views, contracts and AI calls sit on a core that stands on its own.
   The target is a core good enough for systems code (parsers, stores, type checkers) and for
   Eyelang's own compiler. That is not reached yet: of the compiler, only the lexer is written in
   Eyelang (`testdata/programs/lexer`), and self-hosting comes after v0.4.
10. **Agents are first-class users.** Coding agents write much of the code, so the language stays
    small and predictable. One obvious way to do each thing, strict types, canonical formatting,
    explicit signatures and tests that replay recorded replies make their work checkable, and the
    same things make it pleasant for people.
