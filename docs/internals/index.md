---
title: Internals
---

# Internals

How Eyelang is built, for people who want to change it. Start with the architecture; the rest
goes deeper into one part.

- **[How Eyelang works](/internals/architecture)**: the map. The pipeline from source to running
  program, the runtime and its stores, the tools, how a language change flows through the
  repository, and what is generated from what. About ten minutes.
- **[Compiling Eyelang](/internals/compilation)**: the design for native code: the C back end,
  values and reference counting, generic code, and jobs as state machines.
- **[Benchmarks against Go](/internals/benchmarks)**: native code's time and memory against the
  same programs in Go, with the interpreter's beside them. Written by `go run ./tools/bench --write`.
- **[Proposals](/internals/proposals)**: decisions drafted on 2026-10-11, each with the problem,
  options and a recommendation. Every recommendation was accepted, and the decision log in
  SPECS.md records them.

## Reviews

Dated reviews of the language and its implementation. Each describes the code on the day it was
written; later commits may have changed what it found.

- [What the benchmarks say, 2026-10-12](/internals/reviews/benchmarks-review-2026-10-12)
- [Native compilation plan, 2026-10-12](/internals/reviews/native-plan-2026-10-12)
- [How it is to type, 2026-10-12](/internals/reviews/ergonomics-2026-10-12)
- [Performance and packages, 2026-10-12](/internals/reviews/performance-and-packages-2026-10-12)
- [Review conclusions, 2026-10-12](/internals/reviews/conclusions-2026-10-12)
- [Spec review, 2026-10-12](/internals/reviews/spec-review-2026-10-12)

For the language itself, see the [language reference](/reference/language/) and the
[design pages](/design/): goals, the decision log and the open questions.
