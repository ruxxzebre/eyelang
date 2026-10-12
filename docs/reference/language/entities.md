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
- **State and `mut self`:** an entity method is a `mut self` method over the entity's state
  ([section 7](/reference/language/mutation)). A `state` field such as `devices` is short for `self.devices`, and a method is
  called by its name too: `unpin(p)` in another method is `self.unpin(p)`, in the same
  transaction. A pipe still calls a function, never a method: `shapes |> add(s)` is the collection
  function `add`, even in an entity that has a method of that name. The state is
  loaded before the call and saved when the transaction commits; nothing outside the method sees a
  change until the commit.
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
