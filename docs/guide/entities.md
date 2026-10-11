---
title: Entities
---

# Entities

Most programs that run for days keep some shared state: a team's credit, a request's status, a
disk's last reading. Several callers change it, and some of them run at the same time. The usual
tools are a database row and a lock, or an object that many parts of the program hold, and the
usual bugs are lost updates, half-done changes and reads of state that has already moved on.

In Eyelang, shared state lives in an **entity**. An entity has an identity, state that only its
own methods can change, and methods that each run as one transaction. This page explains how to
use entities well, and which mistakes they rule out. It assumes you have read
[An entity](/tutorial/an-entity) in the tutorial.

The code on this page is a [real program](https://github.com/ruxxzebre/eyelang/tree/dev/docs/guide/code/entities)
that the test suite checks and runs.

## An identity and its state

A wallet holds a team's prepaid credit for model calls:

<<< ./code/entities/main.eye#wallet

- `entity Wallet(team: String)` declares the entity and its identity. `Wallet("ops")` and
  `Wallet("research")` are two wallets.
- Each `state` field has a default, which a new wallet starts with.
- A `pub fn` is a method that other code can call. Inside it, the state fields are variables.

`Wallet("ops")` names a wallet; it doesn't create one. An entity exists from its first use, with
its defaults, so there is no "create" step and no "not found" case to handle:

<<< ./code/entities/main.eye#identity

The identity is fixed. A method that assigns to it doesn't compile:

```text
main.eye:4:30: team is the entity's identity and can't change
```

If something about an entity can change, such as a team's display name, make it a `state` field.

## Each method call is one transaction

A method runs with one writer at a time per entity, and all of it commits or none of it does.
Nothing outside the method sees a change until the call returns.

<<< ./code/entities/main.eye#test-wallet

So `spend` can check the balance and then take from it, and no other caller can spend the same
cents in between. You don't write a lock, and you can't forget one: the only way to change a
wallet's state is to call one of its methods.

Put the check and the change in the same method. Code that reads `ops.cents` outside, decides,
and then calls a method that sets a new value has the same race as any read-then-write: two
callers can read the same balance. A method that does both can't.

### Calls to other entities commit together

A method can call methods of other entities. The whole call tree is one transaction:

<<< ./code/entities/main.eye#ledger

<<< ./code/entities/main.eye#test-ledger

`from.spend(n)?` and `to.topUp(n)` change two wallets, and `moves += 1` changes the ledger. Either
all three commit, or, if the call stops on a fault, none of them does. A reader never sees credit
that has left one wallet and not yet reached the other.

::: warning An `Err` is a value, not a rollback
Returning `Err` from a method doesn't undo what it changed: the method ends normally, and its
changes commit. Only a fault, such as `unreachable`, rolls a transaction back. So check first and
change after, as `spend` does: it returns `Short` before it touches `cents`, and `move` calls
`spend` before `topUp`.
:::

### Methods never wait

A method holds its entity while it runs, so it can't pause. It can't sleep, wait for an event or
call another service. A method with `sleep!` in it doesn't compile:

```text
main.eye:109:7: an entity method is a transaction and can't wait; start a job for this instead
```

Because methods have no outside effects, the runtime can safely run one again after a conflict.
Work that waits belongs in a [job](/guide/jobs-and-waiting), which the method can start.

## References

`Ref(Wallet)` is the type of a reference to a wallet. A reference is an identity, not a pointer:
you can keep it in a record or a list, pass it to a job, and save it, and it always names the same
wallet. `move` takes two of them. Calling a method through a reference, `from.spend(n)`, is the
same as calling it on `Wallet(...)`, and `from.team` reads the identity.

## Snapshots

`read()` returns a **snapshot**: the entity's identity, its state and a `version` that counts the
transactions that changed it, from 0 for a new entity. The snapshot's type has the entity's name,
so a function can take one:

<<< ./code/entities/main.eye#snapshot

<<< ./code/entities/main.eye#test-snapshot

A snapshot is a plain value. It doesn't change when the entity does, and changing a copy of it
doesn't change the entity. Pass snapshots to code that should look but not touch: `describe` is a
private function here, and it can't change a wallet because it never gets one.

In a [job](/guide/jobs-and-waiting), `read!()` takes a snapshot as a durable step. A snapshot goes
stale as soon as the next transaction commits, so a job that waits reads the entity again after
the wait instead of trusting what it read before.

`Wallet.all` is every wallet, as a collection of snapshots, so the usual pipes work on it:

<<< ./code/entities/main.eye#all

## Starting a job from a method

A method can't wait, but it can start a job that does. The start is part of the transaction:
the run begins only if the method commits.

<<< ./code/entities/main.eye#quota

<<< ./code/entities/main.eye#test-quota

The second `spend` takes the quota below 100 cents and starts `warnLow`. `warned` and the started
run commit together, so the team is warned exactly once: a call that rolls back starts nothing,
and a call that commits can't lose its warning.

With a database and a queue, this is two writes to two systems, and a crash between them leaves a
change without its follow-up, or a follow-up for a change that never happened. The usual fix is an
outbox table and a relay process. Here the transaction covers both.

`start` also takes `after:` (start later), `at:` (start at a time) and `key:` (start once per
key). See [Jobs and waiting](/guide/jobs-and-waiting#starting-runs).

## Changing the shape of stored state

Entities outlive the code that wrote them, so their state has a shape version. `@version(N)`
numbers the shape (1 without it), and every stored row records the version it was saved at.

<<< ./code/entities/main.eye#versions

- **Adding a field with a default** needs no new version. A row without the field gets the default
  when it is loaded. `frozen` was added this way.
- **Any other change**, such as a rename or a new field without a default, raises `@version` by one
  and adds `migrate from` the old version: a pure function from the row as it was stored to the
  state at the next version. `old` is that row. Its fields are read by name, and checked when the
  migration runs.
- **Migrations chain.** A row saved at version 1 of a version-3 entity runs `migrate from 1`, then
  `migrate from 2`, when it is loaded. So an entity needs a migration from each version between its
  oldest rows and the current one.

The checker holds you to the chain:

```text
main.eye:1:1: Account is at @version(3), so it needs migrate from 2: adding a field with a default needs no new version, and any other change needs a migration from the old one
```

and to the result, which must give every field without a default a value:

```text
main.eye:7:5: this Account state has no owner: every field needs a value
```

Here is the change above on a real journal, from a copy of the program whose `main` prints the
account's snapshot. Before the change, `main` added 3 credits. After it, the stored row loads as
300 cents, with `frozen` at its default, and `main` adds 50 more. The `version` in each snapshot
is the transaction count (1, then 2), not the shape version:

```text
{ team: "ops", version: 1, credits: 3 }
{ team: "ops", version: 1, cents: 300, frozen: false }
{ team: "ops", version: 2, cents: 350, frozen: false }
```

`eye run` reads every stored row's version before it starts. If a migration is missing, it
refuses to start, rather than load state it can't read:

```text
eye run: stored entities can't reach this program's versions: 1 Account saved at version 1, and Account has no migrate from 1
```

## When to use an entity

Use an entity for state that **outlives a call and has more than one writer**: a balance, a
request moving through its steps, the last reading of a disk. Its methods are the only way to
change it, and each change is a transaction.

Use a plain value for everything else: a price list, a request's details, a parsed message, the
result of a calculation. Values are immutable, cost nothing to share, and are passed by value,
so nothing changes behind your back. When code only needs to look at an entity, give it a
snapshot.

A few signs that something should be an entity:

- Two parts of the program, or two runs of a job, change it.
- It must survive a restart.
- A job needs to wait for it to change, which a [view](/guide/views-events-schedules) can notice.

## Mistakes this prevents

| The bug | Why it can't happen |
|---|---|
| A lost update: two callers read, both write | A method checks and changes in one transaction, one writer at a time |
| Half a change is visible | Nothing outside a method sees its changes until it commits, nested calls included |
| A change commits, its follow-up is lost | A job started in a method begins only if the method commits |
| A lock held across a network call | Methods can't wait, so they finish quickly |
| "Not found", or two rows for one thing | An entity exists from its first use, and its identity can't change |
| Old rows that the new code can't read | `@version` and `migrate from` are checked, and `eye run` refuses rows it can't migrate |

## See also

- [Entities](/reference/language/entities) in the reference: identity, transactions, snapshots and
  versions.
- [Jobs and waiting](/guide/jobs-and-waiting): the work an entity starts, and how a job reads and
  changes entities with `!`.
- [Views, events and schedules](/guide/views-events-schedules): queries over entities that start
  jobs.
- [Testing with time and recordings](/guide/testing): every test starts with an empty store.
- [Effects](/guide/effects): a public function that reads or changes an entity lists
  `uses {entity Wallet}`.
