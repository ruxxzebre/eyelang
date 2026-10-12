---
title: "Views and triggers"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Views and triggers

```eye
view idleWorlds = World.all
  |> filter(fn w: w.phase == Running && w.players == {})

@on(enters(idleWorlds))
job sleepWhenIdle(w: Ref(World)) { sleep!(20 min) }   # full version in the tour
```

- A view is a named pipeline over entities. The runtime keeps it up to date as entity transactions
  commit. A job reads it as a recorded step, `idleWorlds.read!()` ([section 15](/reference/language/jobs-and-waiting)).
- A view's pipeline stays inside the program, as an entity method does ([section 14](/reference/language/entities)): it is
  computed as each transaction commits, so a call there that uses `net`, `fs`, `process`, `clock`,
  `random`, `env`, `ffi`, `secret` or `wait` is an error that names the effect. Output is allowed.
  A view that reads the clock would change without a commit, which `enters` and `leaves` never see
  (open question 1).
- `enters(view)` starts one run per entity that begins matching; `leaves(view)` starts one per
  entity that stops matching.
- Over entity sets, pipelines run in memory today, over the entities the store holds, with the
  same functions that work on lists. Compiling them to SQL waits for several runners on one
  database (open question 28), since one runner already holds every entity in memory.
