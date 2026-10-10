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

Each page is one section of the language's specification, [SPECS.md](https://github.com/ruxxzebre/eyelang/blob/main/SPECS.md), which is where the rules are written and changed.

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
