---
title: "Entities"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Entities

```eye
@version(3)
entity Player(id: PlayerId) {
  state name: String = ""                    # v3: renamed from nick
  state devices: Set(DeviceId) = {}          # v3: added

  pub fn addDevice(d: DeviceId) -> Result((), DeviceError) {
    if devices |> size >= 5 { return Err(TooManyDevices) }
    devices = devices |> add(d)
    Ok(())
  }

  pub fn retire(ok: Approval) { end(ok) }     # ending an entity needs an Approval

  migrate from 2 (old) {                      # pure function from the old stored shape
    { name: old.nick, devices: {} }
  }
}
```

- **Identity:** `Player(id)` is a reference. The entity exists on first use, starting from
  default state.
- **Methods are atomic transactions.** One writer at a time per entity, all or nothing. A method
  may call methods on other entities, and the whole call tree commits or rolls back together.
  An `Err` is a value like any other: a method that returns one commits what it changed. Only a
  fault ([section 15](/reference/language/jobs-and-waiting)), such as `unreachable`, rolls the call tree back, and with it the jobs it
  started and the events it published, which never happen.
- **Ending an entity needs an Approval.** `end(ok)` in one of the entity's methods ends it when
  the transaction commits. `ok` is an `Approval`, which only `approval!` makes ([section 15](/reference/language/jobs-and-waiting)), so
  retiring an entity takes a person's consent, and `end` without one doesn't compile. The
  `Approval` isn't matched against the entity: it shows that a person said yes, and the store
  keeps its `what` and `by`, with when the entity ended, for the record.
  - **`end` is resolved by context.** In an entity's own method, `end(…)` is the entity's, as its
    methods are, unless a local, a state field or a method of that name is nearer. Elsewhere
    `end` is a string's end position, `s.end()`. `end` can't be called on another entity, which
    ends itself through a method of its own, such as `retire`; the checker says so.
  - **What an ended entity is.** Its state is gone, and the store keeps only that it ended, when,
    and the approval's `what` and `by`. It leaves every view, so `@on(leaves(…))` jobs start, and
    `Player.all` no longer lists it. Its `Ref` stays a valid value whose identity can be read
    (`p.id`), but calling a method on it or reading it (`read()`, a state field) faults:
    `Player("ann") ended at 2026-10-12 09:00 UTC, approved by operator ("Retire ann?"), so it
    can't be read or changed`. The identity stays ended: using it never starts the entity again,
    so a job holding an old reference can't bring it back by mistake. A `leaves` job that the
    end starts gets the reference; what it needs of the state is passed on before the end.
  - What the method changes after `end(ok)` is dropped with the rest of the state, and a later
    call on the entity in the same transaction faults.
- **State and `mut self`:** an entity method is a `mut self` method over the entity's state
  ([section 7](/reference/language/mutation)). A `state` field such as `devices` is short for `self.devices`, and a method is
  called by its name too: `unpin(p)` in another method is `self.unpin(p)`, in the same
  transaction. A pipe still calls a function, never a method: `shapes |> add(s)` is the collection
  function `add`, even in an entity that has a method of that name. The state is
  loaded before the call and saved when the transaction commits; nothing outside the method sees a
  change until the commit.
  - **A method's closures may read state fields**, unlike a `var` ([section 7](/reference/language/mutation)):
    `players |> filter(fn p: p != owner)`. A closure gets each field's value as it was when the
    closure was made, as it gets a `let`'s, and can't change one: `a closure can't change the
    entity's state`.
- **Methods never wait.** They can't use `!`, so they can't call other services; those calls
  belong in jobs. A method can start a job (`wake.start(id)`), and the job begins only if the
  transaction commits.
  Because entity methods have no outside side effects, the runtime can safely retry one after a
  conflict.
- **Reading from a job:** `w.read!()` returns a snapshot, whose type is the entity's name
  (`fn due(me: Traveller)` takes one): the entity's state, its identity and a `version` field, a
  `Version` that counts the transactions that changed the entity: 0 when it is new, then one more
  for each. Snapshots go stale, and commands can name the version they were based on (see
  `@fresh` in [section 17](/reference/language/contracts-talking-to-other-services)). Outside a job, in a test for instance, `w.read()` gives a snapshot without `!`.
  Since the snapshot's `version` is the transactions', a state field can't be named `version`:
  `a snapshot's version counts the transactions that changed Doc, so a state field can't be
  named version: name it revision, or what it counts`.
- **Evolution:** `@version(N)` numbers the shape of an entity's stored state (1 without it), and
  every row records the version it was saved with; it isn't the snapshot's `version`, which counts
  transactions.
  - Adding a field with a default needs no new version: a row without the field gets the default
    when it is loaded, and a field the entity no longer declares is dropped.
  - Any other change, such as a rename or a new field without a default, raises `@version` by one
    and adds `migrate from` the old version: a pure function from the row as it was stored at that
    version to the state at the next one. `old` is that row; the old shape isn't declared anywhere,
    so its fields are read by name and checked when the migration runs. The migration to the
    current version gives the state the entity declares, which the checker holds it to: every
    field without a default, each of its type. An older one gives the next version's shape.
  - Migrations chain. A row saved at version 1 of a version-3 entity runs `migrate from 1`, then
    `migrate from 2`, when it is next loaded, and is saved at version 3 the next time a transaction
    changes it. So an entity has a migration from each version between its oldest one and the
    current one, with no gaps; dropping the oldest migrations retires the rows saved before them.
  - `eye run` reads every stored row's version before it starts, and refuses to start if one is
    newer than the program's or older than its oldest migration: `3 Player saved at version 1, and
    Player has no migrate from 1`.
