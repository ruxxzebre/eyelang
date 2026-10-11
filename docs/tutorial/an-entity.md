---
title: An entity
---

# An entity

So far every value has been local: it lives in a variable, and it's gone when the function returns.
Real programs keep state: stock on a shelf, a user's settings, the phase of a game world. In
Eyelang, shared state lives in **entities**. An entity has an identity, state that only it can
change, and methods that change it one transaction at a time.

The program is [`docs/tutorial/code/an-entity/main.eye`](https://github.com/ruxxzebre/eyelang/blob/dev/docs/tutorial/code/an-entity/main.eye).

## A shelf

Each item in the shop has a shelf: how many are in stock, and how many each order is holding.

<<< ./code/an-entity/main.eye#error

<<< ./code/an-entity/main.eye#shelf

- `entity Shelf(item: String)` declares the entity and its identity. `Shelf("apples")` and
  `Shelf("pears")` are two different shelves.
- `state count: Int = 0` is a field of the entity's state, with the value a new shelf starts with.
- `pub fn` declares a method that code outside the entity can call. Inside a method, the state
  fields are variables: `count -= n` changes the shelf's count.

## Using it

<<< ./code/an-entity/main.eye#test-shelf

`Shelf("apples")` doesn't create anything. It names the shelf whose identity is `"apples"`. An
entity exists from its first use, with the state's defaults, so there is no "create" step and no
"not found" case.

Reading a field, `apples.count`, reads the current state. Calling a method, `apples.hold(...)`,
runs it against the state and saves the result.

<<< ./code/an-entity/main.eye#identity

## Methods are transactions

Each method call is one transaction: one writer at a time per entity, all or nothing. While `hold`
runs, nothing else can change the same shelf. So `hold` can check the count and then take from it
without a race: no other order can take the last apple between the check and the change. Nothing
outside the method sees a change until the call is done.

You don't write locks for this, and you can't forget one. The state belongs to the entity, and
the only way to change it is through its methods.

Methods can't wait. A method that called `sleep!` would hold the shelf for as long as it slept,
so the checker refuses it:

```text
main.eye:13:5: an entity method is a transaction and can't wait; start a job for this instead
```

Waiting is what jobs are for. The [next page](/tutorial/a-job-that-waits) starts one from a method.

## References

`Ref(Shelf)` is the type of a reference to a shelf. A reference is an identity, not a pointer: it
can be stored in a record or a list, sent, and saved, and it always means the same shelf.

<<< ./code/an-entity/main.eye#refs

`holdAll` takes an order's lines and holds each one, collecting the items that are short.
`line.shelf.hold(...)` calls a method through the stored reference, and `line.shelf.item` reads
the shelf's identity.

<<< ./code/an-entity/main.eye#test-refs

`read()` returns a snapshot of the entity: its identity, its state, and a `version` that counts the
transactions that changed it. A snapshot is a plain value. It doesn't change when the entity
does, so it's what you pass to code that should look but not touch.

## Every shelf

`Shelf.all` is every shelf there is, as a collection, so the usual pipes work on it:

<<< ./code/an-entity/main.eye#all

A program that needs such a list often, such as "every shelf that is running low", declares a
[view](/reference/language/views-and-triggers): a query the runtime keeps up to date as
transactions commit, and that can start a job whenever an entity enters it.

## Next

[A job that waits](/tutorial/a-job-that-waits): a hold that ends by itself after fifteen minutes.

The full rules:

- [Entities](/reference/language/entities): identity, transactions, snapshots and versions, and how
  an entity's stored state changes shape over time.
- [Mutation](/reference/language/mutation): entity methods are `mut self` methods over the state.
- [Effects and signatures](/reference/language/effects-and-signatures): reading or changing an
  entity is the effect `entity Shelf`, which public functions list.
