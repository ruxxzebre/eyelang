---
title: Talking to other services
---

# Talking to other services

A program that charges a card, sends a message or grants access depends on another service, and
calls to other services fail in ways that matter. A timeout after the request went out doesn't say
whether the card was charged. Retry, and the customer may pay twice. Give up, and the invoice may
stay open although the money arrived. Most HTTP clients leave that decision to each place that
makes a call, so it is made differently in each place, and usually only after an incident.

A **contract** describes another service once: its operations, their types, how they go over HTTP,
and what to do when a reply is lost. The compiler holds every call to it, and every test of it, to
that description.

The code on this page is a [real program](https://github.com/ruxxzebre/eyelang/tree/dev/docs/guide/code/contracts):
invoices that charge a customer's card through a payment service and text them a receipt. Its tests
check what this page says:

```sh
eye test docs/guide/code/contracts
```

```text
ok    a charge that goes through marks the invoice paid (548µs)
ok    a declined card is an outcome, not an error (180µs)
ok    a lost reply is settled by asking chargeFor, not by charging again (198µs)
ok    a text whose reply was lost goes to a person, not out again (215µs)
ok    a refund event marks the invoice refunded (262µs)
ok    a failure the query declares comes back as a value (52µs)
ok    the served contract bills once per key (217µs)
ok    values have one JSON shape (16µs)
ok    a status page is read from its status and body (3µs)
9 tests passed (1.691ms)
```

```text
contracts/
  eye.toml                   marks the program's root
  main.eye                   invoices, the jobs, main, and the tests
  payments/payments.eye      contract: the payment service
  sms/sms.eye                contract: a text message gateway
  invoices/invoices.eye      contract: this program's own interface, which it serves
  recordings/                answers from the services, replayed by the tests
```

## A contract describes a service

A contract lives in a module of its own, a directory, next to the types its operations use:

<<< ./code/contracts/payments/payments.eye#types

<<< ./code/contracts/payments/payments.eye#contract

A program that imports the module calls the operations as the module's functions:
`payments.charge!(…)`, `payments.balance(…)`. The types come from the contract, so a wrong argument
or a missing case is a compile error, not a 400 in production.

The `##` comments are the service's documentation. They show on hover in your editor and in
`eye doc`, so the people who call the service read the same words as the people who wrote it.

## Commands, queries and events

Each operation is one of three kinds, and the kind decides what the runtime may do when something
goes wrong.

| Kind | What it is | When the service can't be reached |
|---|---|---|
| `command` | Changes something: takes money, sends a message | Sent again only if it never went out. A lost reply is `Uncertain(key)` (below) |
| `query` | Reads something, and is safe to repeat | Tried again for about a minute while the run waits, then the run ends as `Failed(Unavailable(…))` |
| `event` | Something that happened in the other service | Arrives on its own; starts or wakes runs |

A query whose caller wants to handle failures itself says so in its result type.
`balance` returns `Result(Int, http.HttpError)`, so a failure comes back as an `Err` at once,
instead of being retried:

<<< ./code/contracts/main.eye#balance

<<< ./code/contracts/main.eye#test-balance

An **event** starts a job with `@on(event(…))`. `@key(charge)` names the field that tells one event
from another, so a refund that is delivered twice starts one run:

<<< ./code/contracts/main.eye#refunded

<<< ./code/contracts/main.eye#test-refund

::: info Not yet
Under `eye run`, nothing outside the program can publish an event yet. Tests publish them with
`payments.refunded.publish(…)`, and a program publishes its own.
:::

## Keys make repeats safe

`@key(required)` means every call of the command carries a key, and the service remembers what it
did with each key. Asking again with the same key gives the first answer and does nothing twice.
Leave the key out and the checker stops you:

```text
main.eye:51:33: charge needs an argument for key
```

Choose the key from your own data, so that the same piece of work always gets the same key. The
job below charges `invoice-{id}`: however often that step runs, after a restart or a lost reply,
the card is charged once for each invoice.

<<< ./code/contracts/main.eye#collect

A key made from the clock or a random number defeats this, since each retry gets a new key.

## When the reply is lost: Uncertain

Calls go wrong in two ways, and only one of them is uncertain:

- **Never sent.** The connection was refused, or the name didn't resolve. The service can't have
  done anything, so the request is sent again, command or query.
- **Sent, and the reply was lost.** A timeout, a broken connection, or a 5xx answer to a command.
  The service may or may not have acted. The command's result is `Uncertain(key)`.

The compiler treats `Uncertain` as one more outcome of every command. Until you deal with it, the
result can't be used as the declared type:

```text
main.eye:51:33: this command's reply may have been lost, so it may be Uncertain(key): reconcile it with .reconcile!(), or match it with an Uncertain(_) arm
```

and a `match` that forgets it isn't exhaustive:

```text
main.eye:52:3: this match doesn't handle Uncertain(_): add an arm for it, or `_ => …`
```

Nor can a command's outcome be dropped. With `payments.charge!(…)` on a line of its own, whose
outcome nothing reads, `eye check` says:

```text
main.eye:51:3: this drops Charge, a command's outcome, which may be Uncertain, so a refusal would go unnoticed: match it, or discard it with let _ = …
```

For a command whose answer really changes nothing, such as a notice sent on the way to failing
anyway, say so with `let _ = …`.

There are three ways to settle it. The contract picks the way, once, for every caller.

**Ask a query.** `@reconcile(chargeFor)` names a query that takes the key and returns an `Option`
of the command's outcome. `.reconcile!()` asks it: `Some` is the outcome, and `None` means the
charge never arrived, so it is sent again with the same key. The recording below loses the reply
to `invoice-i-3`, and the test checks that the program asks `chargeFor` and finds the charge,
without charging again:

<<< ./code/contracts/main.eye#test-lost

**Send again.** A service that keeps what each key did can answer a repeat without acting twice.
Then the command reconciles through itself: `@reconcile(issue)` on the invoices contract below, or
`@reconcile(send)` on Notify in the [approvals tutorial](/tutorial/approvals).

**Decide yourself.** The text message gateway takes no key and can't say what it did. Its command
has no `@reconcile`, so it has no `.reconcile!()` either. The caller matches `Uncertain(_)` and
decides, here by asking a person to check, because sending again could send a second text:

<<< ./code/contracts/sms/sms.eye#contract

<<< ./code/contracts/main.eye#receipt

Calling `.reconcile!()` on it is a compile error:

```text
main.eye:68:9: this command names no @reconcile query, so its Uncertain outcome can't be reconciled: match Uncertain(key) instead
```

Nothing is ever sent again on a guess. If a query still can't settle a command after three tries,
the run stops with a fault rather than guess. A command whose reply was lost is kept in the
program's journal until it is settled, so a restart in the middle of reconciling carries on.

## When the state may have changed: @fresh

A job that reads a service's state, decides, and then acts can act on state that changed in
between: it clears a shelf that someone filled again a second ago. `@fresh` on a command makes
every caller say which version of the state it read, and lets the service refuse:

```eye
pub type Shelf = { id: String, items: List(String), version: Version }

@port(3004)
contract Shelves {
  query shelf(id: String) -> Shelf

  ## Empties a shelf, unless it changed after `basedOn`.
  @fresh
  command clear(id: String) -> Cleared
}
```

`clear` now takes `basedOn: Version` after its own parameters, and has two more outcomes:
`Stale`, when the shelf changed after that version, and `Expired`, when the shelf is gone. Neither
did anything. The caller handles both, and usually reads again after `Stale`:

```eye
job tidy(id: String) {
  while true {
    let shelf = shelves.shelf!(id)
    match shelves.clear!(id, basedOn: shelf.version) {
      Cleared(n) => {
        Log(id).add!("cleared {n}")
        return
      }
      Stale => ()
      Expired | Uncertain(_) => return
    }
  }
}
```

Leave out `basedOn:`, or a `Stale` or `Expired` arm, and the checker stops you:

```text
main.eye:6:5: this match doesn't handle Expired: add an arm for it, or `_ => …`
main.eye:6:11: clear needs an argument for basedOn: clear is @fresh, so it takes the Version of the state it was based on, as basedOn: snapshot.version
```

A `Version` is whatever the service gives: a number or a string. An entity's snapshot has one in
its `version` field, so a program that serves the contract answers from it:

```eye
impl shelves.Shelves {
  fn shelf(id) {
    let rack = Rack(id).read()
    { id, items: rack.items, version: rack.version }
  }

  fn clear(id, basedOn) {
    let rack = Rack(id).read()
    if rack.gone { return Expired }
    if rack.version != basedOn { return Stale }
    Cleared(count: Rack(id).empty())
  }
}
```

## Commands that need a person: @destructive

`@destructive` on a command adds a parameter `approval: Approval`. Only `approval!` makes an
`Approval`, so the command can be called only after a person said yes:

```eye
job demolish(id: String) {
  match approval!("Burn shelf {id}?", from: "operator", within: 1 h) {
    Approved(ok) => {
      let _ = shelves.burn!(id, approval: ok)
    }
    Declined | Expired => ()
  }
}
```

```text
main.eye:17:15: burn needs an argument for approval: burn is @destructive, so it takes the Approval that approval!(…) gives
```

The [shelves program](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/programs/contracts)
serves and calls both commands, and its tests show each outcome.

## Over HTTP

Attributes say how each operation goes over HTTP. The address and the credentials are never in the
code: they come from the environment when the program runs (see
[Running in production](/guide/running#configuration)).

| Attribute | What it does |
|---|---|
| `@http("POST /v1/charges")` | The method and path. A path segment `:name` takes the argument of that name |
| | `GET` and `DELETE` send the other arguments as query parameters, a list as the parameter repeated; other methods send them as a JSON object |
| `@http(…, status: { 201: Charged, 402: Declined })` | A reply with that status is read as that variant, from the fields of the body. Any other success is read as the result type |
| `@http(names: snake)` on the contract | Names are snake_case on the wire: `amountCents` is `amount_cents`, and a variant `Normal` is `"normal"` |
| `@key(required)` | The key goes in the `Idempotency-Key` header. `header: "…"` names another header, and `field: "…"` puts the key in the body too |
| `@auth(bearer: "payments-token")` on the contract | Sends `Authorization: Bearer` and the secret `payments-token` with every request |
| `@auth(header: "X-Auth-Token", secret: "…")` | Sends a secret in a header of your choice |

Arguments are sent with their defaults filled in. An argument that is `None` is left out.
`basedOn` and `approval` go like any other argument (`based_on` with `names: snake`).

This is what the payment service received when the program ran against a stand-in, with
`PAYMENTS_URL` and `PAYMENTS_TOKEN` set:

```text
POST /v1/charges
Authorization: Bearer pt-secret
Idempotency-Key: invoice-i-1
Content-Type: application/json
{"amount_cents":4250,"customer":"ann","description":"Invoice i-1"}
```

How the answer is read depends on its status, and on whether the operation is a command, a query,
or a query that returns `Result(T, http.HttpError)`:

| The service | Command | Query | Query returning `http.HttpError` |
|---|---|---|---|
| Answers with a status named in `status:` | That variant | That variant | `Ok` of that variant |
| Answers with another 2xx | The body, read as the result type | The same | `Ok` of the body |
| Answers with another 4xx | A fault that ends the run | A fault | `Err(BadStatus(code, body))` |
| Answers with a 5xx | `Uncertain(key)` | Tried again | `Err(BadStatus(code, body))` |
| Answers with 429 | Tried again | Tried again | `Err(BadStatus(code, body))` |
| Can't be reached: the connection is refused | Tried again | Tried again | `Err(NoAnswer(reason))` |
| Doesn't answer: a timeout or a broken connection | `Uncertain(key)` | Tried again | `Err(TimedOut)` or `Err(NoAnswer(reason))` |

"Tried again" means for about a minute, while the run waits; after that the run ends as
`Failed(Unavailable(…))`. A `@fresh` command reads a 409 as `Stale` and a 410 as `Expired`,
unless `status:` names those codes; `status: { 412: Stale }` names another.

A refused request is a bug in the program or its configuration, not an outcome to handle, so it
stops the run where you can see it:

```text
main.eye:51:16: job collect: payments.charge was refused with status 404: {"error": "no such route"}
```

## Values as JSON

Every value that leaves a program as JSON has one shape: in requests and replies, in recordings, in
the schemas model calls use, and in `toJson`. The test `values have one JSON shape` checks the
examples below.

| Eyelang | JSON |
|---|---|
| A record | An object with every field: `{"at": "…", "every": 90000, "level": "Busy", "note": null}` |
| A variant of a type whose variants have no fields | A string: `"Busy"` |
| Any other variant | An object tagged with `"kind"`, with its fields: `{"kind": "Charged", "id": "ch-1", "at": "…"}` |
| A positional field | `"value"`, or `"value1"`, `"value2"`, … when there are several |
| A type that wraps one value, such as `ChargeId("ch-1")` | That value: `"ch-1"` |
| `Some(x)`, `None` | `x`, `null` |
| Lists, sets and tuples; a map with string keys | Arrays; an object |
| `Instant`, `Date` | `"2026-10-12T09:30:00Z"`, `"2026-10-12"` |
| `Duration`, `Size` | A number of milliseconds, a number of bytes |
| `Version`, `Approval` | A number or a string, as the service gave it; `{"what": "…", "by": "…"}` |
| `Stale`, `Expired`, `Uncertain(key)` | Always tagged, whatever the command's own variants: `{"kind": "Stale"}` |

So `Charged(id: ChargeId("ch-1"), at: 2026-10-12 09:30 in "UTC")` goes over the wire as:

```json
{ "kind": "Charged", "id": "ch-1", "at": "2026-10-12T09:30:00Z" }
```

Reading is forgiving in two ways, so a service can grow without breaking its callers: fields a type
doesn't declare are ignored, and a missing field of an `Option` type is `None`. Anything else that
doesn't fit, such as a missing field or a string where a number belongs, stops the run with a
fault that names the field.

## Implementing the other side

**In Eyelang,** a program serves a contract with `impl`: one function per operation, named after
it, with its parameters in order and the key last for a command with `@key`. The types and defaults
come from the contract, so they aren't written again. Here the invoices program serves its own
interface:

<<< ./code/contracts/invoices/invoices.eye#contract

<<< ./code/contracts/main.eye#serve

Each operation runs as one transaction, as an entity method does: it can't wait, and it may start
jobs. The key names the invoice, so a repeated `issue` finds the invoice it already made, with no
separate table of keys:

<<< ./code/contracts/main.eye#test-serve

A program that serves a contract calls its own implementation, unless the service's URL is set in
the environment; then the service is elsewhere.

`@port(8080)` says where the service listens. A contract that a program serves must have one, and
a contract without one, such as the payment service's, is a service outside your network, reached
at the URL in its configuration. A contract with a port can also say who may call it:
`@allow(shop, billing)` lets only those modules call its operations, and a call from any other
module fails to compile, at the call. The contract's own module, the module that serves it and
tests may always call it.

Under `eye run`, an `impl` also serves other programs, on the contract's port. A request is read
the way an Eyelang caller sends it, so the two sides agree without code of their own:

- The operation's `@http` method and path pick it; one without `@http` is reached only from inside
  the program. A path nothing answers gets 404, and a known path with another method 405.
- Its arguments come from the path, the query parameters or the JSON body. One left out takes its
  default, an `Option` left out is `None`, and anything else missing or of the wrong shape gets 400
  with `{ "error": "…" }`. A command with `@key` gets 400 without its key header.
- With `@auth`, a request without the secret gets 401 and reaches no code.
- The reply is the result in the JSON shape above, with the status that `status:` gives its variant
  (the lowest, when several do), else 200.
- A fault answers 500 and changes nothing, so a caller's command comes back `Uncertain`, and
  reconciling it sends it again with the same key.

`eye run` keeps no replies of its own: a repeated key reaches the `impl` again, which finds what it
already did through the entity the key names, as the `Invoice` named by the key does above.

::: info Not yet
Types generated from contracts for other languages are planned for v0.3.
:::

**In another language,** a service that an Eyelang program calls needs to do what its contract
says:

- Answer each `@http` route, reading arguments from the path, the query string or a JSON body.
- Read and write values in the JSON shape above, with snake_case names if the contract says
  `names: snake`.
- For a command with `@key`, remember what each key did, and answer a repeated key with the first
  answer, without acting again.
- For `@reconcile(query)`, offer that query: the outcome of the command with that key, or `null` if
  it never arrived.
- For a `@fresh` command, compare `basedOn` with the current version, and answer 409 without
  acting when they differ, or 410 when what it acts on is gone.
- For a `@destructive` command, record the `approval` it carries. You can't check it, so let only
  the callers in `@allow` reach the service.
- Answer a bad request with a 4xx, which stops the caller's run, and a passing problem with a 5xx,
  which a query retries and a command treats as uncertain. Don't answer a command with a 5xx after
  acting on it, unless a repeat with the same key gives the real answer.

## Services without a contract

Some services have no contract: a public web page, a status page, a feed. The standard `http`
module reads them:

<<< ./code/contracts/main.eye#status-page

`http.get!` returns `Result(http.Response, http.HttpError)`. Any answer is `Ok`, whatever its
status, with the status, the headers (names in lower case) and the body as text. `Err` means there
was no answer: `NoAnswer(reason)` or `TimedOut`. `via:` is the URL of an HTTP proxy to go through.
`serviceUp` has `!` steps, so it runs only in a run, and every outside call in it is a step, the
read of `PAYMENTS_STATUS_URL` included: `env.get!(…)`.

Prefer a contract for any service you call more than once. `http.get` has the broad effect `net`
rather than `net Payments.balance`, it has no keys or reconciling, and its answers have no types.

::: info Not yet
`http.get` has no recordings, so a test can't replay it. Keep the reading in a pure function, like
`looksUp`, and test that with a `Response` you build:

<<< ./code/contracts/main.eye#test-page
:::

## Testing with recordings

Tests don't call real services. `with fakes(…)` answers each call from a recording of a real
answer, one JSON file per contract module, named after the module:

<<< ./code/contracts/recordings/payments.json

<<< ./code/contracts/main.eye#test-charged

- An exchange is found by its operation and, for a call with a key, the key. A call without a key
  gets the first exchange for its operation.
- Each result is in the JSON shape above, and is checked against the contract when the fakes are
  applied, so a recording that no longer fits the contract fails the test that uses it.
- A lost reply is recorded as `{ "kind": "Uncertain", "key": "…" }`. That is how the test of
  `invoice-i-3` loses a reply on purpose.
- Keep other scenarios in other directories. The test of a lost text uses
  `with fakes(sms, from: "recordings/lost-text/")` and the usual payment recordings.

A call that no recording answers stops the run, and the test fails with the operation and key it
was looking for.

::: warning
A test without fakes for a contract calls the real service if its URL variable, such as
`PAYMENTS_URL`, is set in the environment. Give every test that reaches another service its fakes.
:::

See [Testing with time and recordings](/guide/testing) for clocks, events and approvals in tests.

## What this rules out

- **Charging twice.** A command is never sent again on a guess. A lost reply is settled by asking,
  or by a repeat that the key makes harmless, or by a person.
- **Ignoring a lost reply.** `Uncertain` is part of every command's result type, and `match` must
  handle it.
- **Acting on state that changed.** A `@fresh` command names the version it was based on, and
  the caller must handle `Stale` and `Expired`.
- **Destroying without a person.** A `@destructive` command needs an `Approval`, which only
  `approval!` makes.
- **Credentials in code.** Addresses and secrets come from the environment; the contract names
  only the secret.
- **Drift between caller and service.** The types, routes and keys are written once, and
  recordings are checked against them.

## See also

- [Contracts: talking to other services](/reference/language/contracts-talking-to-other-services), the exact rules.
- [All together: approvals](/tutorial/approvals), contracts in a larger program.
- [Effects](/guide/effects): why `net Payments.balance` appears in signatures.
- [Running in production](/guide/running): `PAYMENTS_URL`, secrets and `$EYE_SECRETS`.
- [Testing with time and recordings](/guide/testing).
- API: [http](/reference/api/std/http), [prelude](/reference/api/std/prelude).
