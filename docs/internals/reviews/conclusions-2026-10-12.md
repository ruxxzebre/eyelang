# Conclusions from the reviews of 2026-10-12

What the coordinating session concluded from the [spec review](./spec-review-2026-10-12.md),
and what it proposes to do about each finding. The performance and package-manager analysis is
still running; its report and conclusions are added here when it lands.

## The short version

The spec is in good shape as a design and in poor shape as a reference. Its ideas hold: none of
the findings argues against a pillar. But the code has moved faster than the text. Four findings
are correctness bugs that let an ordinary program do the wrong thing without a word, and they
come first. After them come a handful of language decisions that only the owner can make, then
one sweep of the text.

## 1. Correctness bugs: fix first

These break a pillar in programs that look right. Each becomes a task with an issue; none needs
a new idea, only the rule SPECS already implies, enforced.

| # | Finding | Proposed rule | Task |
|---|---|---|---|
| 1 | Entity methods and served operations can call services, write files, read the clock and sleep | The checker refuses `net`, `fs`, `process`, `clock`, `random`, `env`, `ffi`, `secret` and `wait` in entity methods and `impl Contract` functions; starting a job and publishing stay, since they commit with the transaction. §12's sentence that says otherwise goes. | G.7 |
| 2 | A job reads `ref.field`, a view or `E.all` without a step, so a resumed run can see a different value | In a job these are errors that say to read with `w.read!()` (or, for views, a recorded `read!`). Outside jobs `ref.field` is short for `ref.read().field`, written into §14. | G.8 |
| 3 | A call tree that comes back to an entity loses a write | The whole call tree is one transaction, so a call back into an entity already in it sees and updates the same in-progress state. (The alternative, refusing re-entry, is simpler but rejects programs that §14's "the whole call tree commits together" invites.) | G.9 |
| 4 | Effects missing at the boundary: `pub` entity methods and `pub` jobs need no `uses`; starting a job isn't an effect; calling a waiting function without `!` drops `wait`; the key rule for jobs started from `main` doesn't follow calls | Entity methods and jobs that are `pub` state their effects as `pub fn` does (`eye fmt` writes them); a new effect `start J` names each job a function starts, which also lets the key rule follow calls; outside a job, calling a function that waits keeps `wait` in the caller's effects. | G.10 |

The `end(ok)` collision and the missing rules for an ended entity (finding 6) belong to C.1,
which is being built now; the worker was told.

## 2. Decisions for the owner

Each of these changes what ordinary programs look like, so it goes to the owner first. The
recommendation is given; nothing is built until the owner agrees.

**Decided by the owner on 2026-10-12:** 1 (results and outcomes can't be dropped silently: task
G.12, #95) and 5 (the waits are named `allDone!` and `firstDone!`, built by C.2). The owner also
confirmed C.8's choices: text that matches no waiting approval is refused, an answer with no
`by` comes from the person asked, and who may publish an event is open question 35. Decisions
2, 3, 4 and 6 are still open.

1. **Results and outcomes can't be dropped silently** (finding 7). Recommend yes: a statement
   whose value is a `Result` or a command's outcome must be used, matched or discarded with
   `let _ = …`. It is the easy-but-wrong program "How to decide" item 1 is about, and the tour
   itself has it. Cost: a `let _ =` where a program means to ignore a failure.
2. **Reading a `var` in a callback** (design risk 4). Recommend yes, for callbacks only (a
   function given to a parameter, which can't outlive the call): it reads the variable's value
   at the call. Stored functions keep the rule. Under value semantics this can't alias, and the
   loop it unblocks is one of the commonest shapes there is.
3. **Record literals in lambdas and pipelines** (design risk 3, open question 25). Recommend
   letting a lambda's body take its record type from where the result goes, which is the rule
   literals already follow, rather than new syntax at the literal. If that isn't enough, then
   `User { … }`.
4. **Built-in collection functions as methods** (finding 10). `xs.first()` and `xs |> first`
   both work today, against "one obvious way". Recommend keeping only the pipe form and letting
   `eye fix` rewrite the method form, or else writing the method form into SPECS as the rule for
   every function whose first parameter is the receiver. Either is defensible; two spellings for
   built-ins only is not.
5. **Overloaded names** (design risk 11). Recommend renaming the waits that clash with
   collection functions (`all!` and `first!` resolve to `all` and `first` today) before C.2
   builds them, and adding a glossary to SPECS. C.2 is being built now, so this needs an answer
   soon; until then the worker builds them under their spec names.
6. **Division by `NonZero`** (design risk 5). Recommend keeping the rule and adding the helpers
   that make the common cases short (`average`, and `div` returning an `Option`), rather than
   relaxing it.
7. **Events have no caller guard** (gap, and design risk 7). Recommend an open question first:
   who may publish an event, and whether approval answers need a typed identity from the
   contract that delivers the press rather than text. C.8's worker was asked to write this into
   SPECS as an open question rather than build a mechanism.

## 3. The text: one sweep

A single task (G.11) that changes no behaviour:

- the stale rows and paragraphs of finding 8 (SQL, resume from the start, checkpoints at every
  `!`, the `Store` interface, Q24's premise, `@priority`, row 117's reference, §21);
- a short rule for waiting functions (finding 5), which open question 23 then builds on;
- the rules the code chose and SPECS never recorded (finding 10): unprefixed imported variants,
  state fields captured in entity lambdas, a `state version` field (make it an error), faults
  beyond the six listed, which standard modules need `import`, how ranges end and wrap;
- the contradictions in the full list (function types without `uses`, the newline and brace
  lists, impls of a contract from another module, ports against `NOTIFY_URL`);
- the open questions: mark each decided one "Decided: see the decision log" in place, keeping
  its number, and keep only the residual part open (numbers never change again);
- a glossary (checkpoint, run, step, key, version, handler);
- the decision log split into language and runtime/tooling, with lab histories moved to the
  design pages, and implementation detail (buckets, advisory locks, environment variables) moved
  to `docs/internals`.

The broken examples of finding 9 are fixed by C.7, which makes every SPECS example check in a
context; that task is next in the queue.

## 4. What is already done

Found by the review and fixed the same day, or fixed by work that was already under way:
`Type.f()` through another module (G.4), a type alias that names itself (G.5), `impl` on an
entity (G.6), `cron` and the schedule policies (C.3), `@fresh` and `@destructive`, which also
gave `Version` a type (6.3), and serving a contract over HTTP (6.13).

## 5. Next steps

1. File G.7 to G.11 as issues under "Core gaps", and build G.7 to G.10 (the correctness bugs)
   in the next wave, alongside the tasks already queued.
2. Bring the seven decisions above to the owner.
3. Add the performance and package-manager report and its conclusions to this folder when it
   lands.
