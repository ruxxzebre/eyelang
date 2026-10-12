---
title: "Language reference"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Language reference

Eyelang 2 is a general-purpose, statically typed, compiled language for **software that keeps state
and waits**: services, automations, schedulers and agents that run for days, hold data, call other
systems that fail, and spend most of their time waiting. It is designed to make such programs as
simple to write as a script and correct by construction. Typically that means data spread across
many entities (users, devices, game worlds, apps) and jobs that run briefly and then wait for
minutes, hours or days. It is not a configuration DSL: every feature below is an ordinary language
construct that also works outside that domain.

## Glossary

Words this document uses in one sense, and names that mean different things in different places.

| Word | Meaning |
|---|---|
| run | One execution of a job, from its start to its outcome. Each start makes one, unless its key started one already; `Run(a)` is the value that names it ([section 15](/reference/language/jobs-and-waiting)). |
| step | A `!` call in a job. It runs once, and its result is recorded in the run, so a resumed run gets the recorded result instead of calling again ([section 12](/reference/language/effects-and-signatures)). |
| checkpoint | A run's saved place: its live variables, the statement it is at, and where each loop around it is. The interpreter takes one at a statement of the job's body once steps have been taken since the last; a run that wakes restores the latest and replays only the steps since ([section 15](/reference/language/jobs-and-waiting)). `zeropod.checkpoint!` in the tour is an operation of a service, not this. |
| transaction | What runs all or nothing: an entity method's call tree, a served operation, or a job's step that calls an entity. Jobs it starts and events it publishes happen only if it commits ([section 14](/reference/language/entities)). |
| key | An idempotency key, a `Key`: what tells one command, job start or event from a repeat of it, so the same key never does the work twice (`key:` on a call, `@key` on an operation or event). A map's keys are ordinary values. |
| version | A snapshot's `version` is a `Version` that counts the transactions that changed the entity, and is what a `@fresh` command's `basedOn` takes (sections 14 and 17). `@version(N)` numbers the shape of an entity's stored state, for migrations. A state field can't be named `version`. |
| handler | What `with` applies to the rest of a test's block: `clock(…)`, of the built-in type `Handler`, and `fakes(…)`, of the built-in type `Fakes`, which also lists the calls it answered ([section 20](/reference/language/time-secrets-tests)). A program may declare its own `Handler`, as [section 12](/reference/language/effects-and-signatures)'s example does; in its module the name is the program's. |
| outcome | A run's `Outcome`, `Completed(value)` or `Failed(fault)` ([section 15](/reference/language/jobs-and-waiting)); a command's outcome is the variant its result is ([section 17](/reference/language/contracts-talking-to-other-services)). |
| fault | What ends a run that code can't catch, such as `DeadlinePassed` or `Unreachable` ([section 15](/reference/language/jobs-and-waiting)). An `Err` is a value, not a fault. |
| method | What `x.f(a)` calls: a method of `x`'s type, from its `impl`, an entity's or a trait's; else a field of `x` that holds a function; else a function in scope whose first parameter takes `x`, as `f(x, a)` ([section 8](/reference/language/methods-and-generics)). |
| callback | A function given to a parameter whose function type has no `uses`, as `filter`'s. It can't be kept, so it runs before the call returns: its effects count where it is given ([section 12](/reference/language/effects-and-signatures)), and a lambda given as one may read a `var` ([section 7](/reference/language/mutation)). |
| `end` | In an entity's own method, `end(ok)` ends the entity ([section 14](/reference/language/entities)). Elsewhere `s.end()` is a string's end position, and `r.end` a range's end. |
| `start` | `j.start(…)` starts a run of job `j`, and `claude.start!(…)` an agent's session. `s.start()` is a string's first position, and `r.start` a range's start. |
| `first` | `first(xs)` is a collection's first element, an `Option`. Waiting for whichever of several runs finishes first is `firstDone!`, and for all of them `allDone!`, named apart from `first` and `all` ([section 15](/reference/language/jobs-and-waiting)). |
| `join` | `xs \|> join(", ")` joins strings; `run.join!()` waits for a run's outcome, in a job. A test or `main` reads the outcome without waiting, with `run.outcome()`, and `run.join()` on a run is an error that says so. |

Each page is one section of the language's specification, [SPECS.md](https://github.com/ruxxzebre/eyelang/blob/dev/SPECS.md), which is where the rules are written and changed.

- [A tour](/reference/language/a-tour)
- [Lexical structure](/reference/language/lexical-structure)
- [Values and types](/reference/language/values-and-types)
- [Bindings, functions and pipes](/reference/language/bindings-functions-and-pipes)
- [Mutation](/reference/language/mutation)
- [Methods and generics](/reference/language/methods-and-generics)
- [Pattern matching](/reference/language/pattern-matching)
- [Errors: Result only](/reference/language/errors-result-only)
- [Traits](/reference/language/traits)
- [Effects and signatures](/reference/language/effects-and-signatures)
- [Programs and concurrency](/reference/language/programs-and-concurrency)
- [Entities](/reference/language/entities)
- [Jobs and waiting](/reference/language/jobs-and-waiting)
- [Views and triggers](/reference/language/views-and-triggers)
- [Contracts: talking to other services](/reference/language/contracts-talking-to-other-services)
- [Calling C and Go](/reference/language/calling-c-and-go)
- [AI models and agents](/reference/language/ai-models-and-agents)
- [Time, secrets, tests](/reference/language/time-secrets-tests)
- [Grammar sketch](/reference/language/grammar-sketch)
