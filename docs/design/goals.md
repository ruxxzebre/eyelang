---
title: "Goals"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Goals

- **Good programs are the point.** A feature earns its place by the programs written with it:
  correct, fast, lean on memory, well organised and easy to change. The obvious way to write
  something should be the right way. [CLAUDE.md](https://github.com/ruxxzebre/eyelang/blob/dev/CLAUDE.md) turns this into a guide for decisions.
- **Concise and pleasant.** Little ceremony, inference inside bodies, one obvious way to do each
  thing, and errors, formatting and editor support that ship with every feature.
- **A good general language first.** Values with no shared mutation, variants with exhaustive
  matching, traits and generics, inferred types, structured concurrency, and no nulls or exceptions.
  The domain constructs (entities, jobs, views, contracts) sit on top of a core that stands on its
  own.
- **Good enough to host itself.** The core must be comfortable for systems code: parsers, stores,
  type checkers. Eyelang's own compiler should be writable in Eyelang once it compiles to native
  code (v0.4).
- **Waiting is cheap.** A job that is waiting is a saved record in the store, not a process or a
  thread. Resuming one takes milliseconds.
- **State has an owner.** Every piece of stored state belongs to exactly one entity. Changes inside
  eyelang are atomic, and changes to other services are keyed and reconciled, never blindly retried.
- **Everything a reviewer needs is in the diff.** Public signatures state their types and effects,
  and every point where a job can pause is marked with `!`.
- **Small and predictable for agents.** Coding agents write most of the changes, so there is one
  obvious way to do each thing, strict checks, and tests that can't flake.
- **Low memory.** Compiled to native code with reference counting (no tracing GC). A waiting job
  holds no thread, stack or record in memory: the journal and the Postgres store keep its record
  on disk and only its handle, about 140 bytes, in RAM ([section 15](/reference/language/jobs-and-waiting)).
