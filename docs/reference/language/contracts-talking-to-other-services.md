---
title: "Contracts: talking to other services"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Contracts: talking to other services

A contract is the single description of another service's interface. It is written once and shared
by eyelang and the Go, TypeScript, Rust and Python code that implements or calls it. Think of it as a
trait (an interface of operations) plus the facts needed to deploy it: port, allowed callers, keys and
how to reconcile.

```eye
@port(3001)
@allow(digest, inbox, welcome)          # who may call; everyone else is denied
contract Notify {
  @key(required)
  @reconcile(receipt)
  command send(to: UserId, text: String) -> Delivery

  query receipt(key: Key) -> Option(Delivery)
  query identity(user: UserId) -> Identity

  @key(receipt)
  event delivered: Delivered on "notify.events"
}

type Delivery =
  | Accepted(receipt: ReceiptId, at: Instant)  # the outbox has it; quiet hours may hold it
  | Pending(receipt: ReceiptId)
  | Refused(reason: Refusal)
```

- **`import notify` is a typed edge.** Port, network policy, credentials and the allowed-caller check
  are derived from it, so a wrong port or an unapproved caller is a compile error.
- **`@port(n)` is where the service listens.** A program that serves a contract with `impl`
  serves it on that port, so a contract that a program serves must declare one: an `impl` of a
  contract without `@port` is an error at the contract's name. Callers never write a port; they
  get it from the contract, so no call can go to a wrong or undeclared one. A contract without
  `@port` describes a service outside the program's network, such as Telegram or a hosted feed
  reader: the program reaches it at the URL its configuration gives (below), and any module may
  call it.
- **`@allow(m, …)` lists the modules that may call the contract's commands and queries.** A call
  of an operation, or a use of one as a value, from any other module is an error at the
  operation's name. The contract's own module and the modules that serve it may always call it,
  and so may tests, which reach fakes or the program's own `impl`, never the network. A contract
  without `@allow` may be called by any module.
  - Each name is a module the program can import, loaded or not, so the lab's digest deployed
    alone still names `trains` in Notify's list; a misspelt or removed module is an error at its
    name.
  - `@allow(nobody)` lets no module of the program call it: the program only serves it, to
    callers outside. `nobody` takes no other names beside it.
  - `@allow` needs `@port`: the callers it lists are let through to that port, and nothing else
    reaches it (the network policy that `eye gen policy` writes). A service outside the network
    has no port to guard.

  ```eye
  # notify/notify.eye
  @port(3001)
  @allow(digest, approving)
  contract Notify {
    query identity(user: UserId) -> Identity
  }

  # trains/trains.eye: an error, since trains isn't in Notify's @allow
  fn whoIs(user: UserId) -> Identity = notify.identity(user)
  ```
- **Commands** return a variant type, and every command can also return `Uncertain(key)`: its
  reply was lost, so the other service may or may not have acted. Until it is reconciled, or a
  `match` handles `Uncertain(_)`, the result can't be passed where the declared type is expected. With `@reconcile(query)`,
  `.reconcile!()` resolves it by asking the query, which returns `Option` of the command's outcome:
  `Some` is the outcome, and `None` means the command never arrived, so it is sent again with the
  same key, which can't do it twice. A command without `@reconcile`, such as one to a service that
  takes no idempotency key, has no `.reconcile!()`: its caller matches `Uncertain(key)` and decides,
  usually by stopping for a person. Nothing is ever resent on a guess. A service that keeps what
  each key did, as Notify does, names the command itself: with `@reconcile(send)`, reconciling
  sends it again with the same key, and the service answers without doing it twice.
- **Never sent is not uncertain.** A request that never reached the service (the connection was
  refused, or the name didn't resolve) can't have done anything, so it is sent again, as a query
  would be. Only a request that went out and whose reply was lost (a timeout, a broken connection,
  or a 5xx answer to a command) is `Uncertain(key)`.
- **Queries** are safe to repeat: one that can't reach its service is retried, while the run
  waits, and after a minute the run ends as `Failed(Unavailable(…))`. A query whose failures the
  caller handles declares them in its result: `query page(url: String) -> Result(String, FetchError)`,
  or, over HTTP, `Result(T, http.HttpError)`, which gets the failures instead of retrying.
- **Events** are published with `notify.delivered.publish(d)`: from an entity method or an
  operation when the transaction commits, and from a job as a `!` step, `notify.delivered.publish!(d)`. `@key(field)` on an event
  names the field that tells one event from another, so an event delivered twice starts one run of
  each `@on(event(…))` job.
- **`@fresh`** on a command adds a required `basedOn: Version` argument and `Stale`/`Expired`
  outcomes, so an old command can't act on newer state.
- **`@destructive`** on a command adds a required `Approval` argument.
- **A contract's operations are its module's functions.** A module that declares one contract offers
  its commands and queries by name: `notify.send!(…)` for `contract Notify { command send … }`.
- **Serving a contract.** A program written in Eyelang serves a contract with `impl Contract { … }`:
  one function per operation, named after it, with its parameters in order and `key` last for a
  command with `@key`. Types and defaults come from the contract. Each runs as one transaction, as
  an entity method does, so it can't wait, and it may start jobs. A key can name an entity, so a
  repeated command finds the work it already did:

  ```eye
  impl notify.Notify {
    fn send(to, text, key) = Message(key).queue({ text, to })
    fn receipt(key) = Message(key).delivery()
  }
  ```
- **Values as JSON.** Every value that leaves a program as JSON has one shape: in requests and
  replies over HTTP, in recordings, in model schemas ([section 19](/reference/language/ai-models-and-agents)), in `toJson`, and in the types
  generated for other languages, where a variant type becomes a discriminated union.
  - A record is an object, with every field.
  - A type whose variants have no fields is a string: `Normal` is `"Normal"`.
  - Any other variant is an object tagged with `"kind"`, with its fields:
    `{ "kind": "Accepted", "receipt": "r-41", "at": "…" }`. A positional field is `"value"`, or
    `"value1"`, `"value2"`, … when there are several.
  - A type that wraps one value, such as `| ReceiptId(String)`, is that value.
  - `Some(x)` is `x`, and `None` is `null`; a missing field of an `Option` type is `None`.
  - Lists, sets and tuples are arrays, and a map with string keys is an object.
  - An `Instant` is an RFC 3339 string, a `Date` `"2026-10-12"`, a `Duration` a number of
    milliseconds, and a `Size` a number of bytes.

  Fields that a type doesn't declare are ignored when a value is read, so a service can add some
  without breaking its callers.
- **Over HTTP.** Attributes say how a contract's operations go over HTTP. The address and the
  credentials come from the configuration, never from the code:

  ```eye
  @http(names: snake)                       # dedupeKey is dedupe_key, Normal is "normal"
  @auth(bearer: "notify-token")             # Authorization: Bearer <secret("notify-token")>
  contract Notify {
    @http("POST /v1/send", status: { 200: Accepted, 202: Pending, 400: Refused, 403: Refused })
    @key(required, header: "X-Actions-Idempotency-Key", field: "dedupe_key")
    @reconcile(send)
    command send(event: String, title: String, priority: Priority = Normal,
      to: Option(List(UserId)) = None) -> Delivery
  }
  ```

  - `@http("METHOD /path")` on an operation gives its request. A path segment `:name` takes the
    argument of that name, `GET` and `DELETE` send the other arguments as query parameters (a list
    as the parameter repeated), and the other methods send them as a JSON object.
  - Arguments are sent with their defaults filled in; a `None` is left out, which is how JSON says
    a value is absent.
  - `@key(…, header: "…")` names the header that carries the key (`Idempotency-Key` without one),
    and `field: "…"` puts it in the body too.
  - `status: { 202: Pending }` reads a reply with that status as that variant, from the fields of
    the body. Any other success is read as the operation's result type.
  - `@auth(bearer: "secret")`, or `@auth(header: "X-Auth-Token", secret: "secret")`, sends a
    secret with every request.
  - A refused request (a 4xx that `status:` doesn't name) ends the run with a fault that shows the
    reply: it is a bug in the program or its configuration, not an outcome to handle.

  A program finds a contract's service at the URL in the environment variable named after its
  module (`NOTIFY_URL` for `notify`, `LAB_NOTIFY_URL` for `lab.notify`). A program that serves a
  contract itself calls its own implementation, unless that variable is set.
- **Fakes come from recordings.** `with fakes(notify, from: "receipts/")` replays real recorded
  exchanges, checked against the contract. Reading a field that no recording contains is a compile
  error, unless it is marked `@unwitnessed("why")`. A recording is a JSON file per contract module,
  named after it (`receipts/notify.json`), that lists exchanges by operation and key, each result
  in the shape above:

  ```json
  { "contract": "Notify",
    "exchanges": [{ "op": "send", "key": "ireland-2026-10-10",
                    "result": { "kind": "Accepted", "receipt": "r-41", "at": "2026-10-10T07:40:02Z" } }] }
  ```

  A command's lost reply is recorded as `{ "kind": "Uncertain", "key": "…" }`. Recordings written
  before this shape was settled, with a variant as its name's only key (`{ "Accepted": {…} }`), are
  still read.
- **A test sees what a fake received.** `fakes(…)` is a value: a test that wants to look names
  it, applies it with `with`, and asks it with `calls(op)`. That lists every call of the operation
  the fakes answered, oldest first, each a record of the call's arguments with their defaults
  filled in, and `key` last for a command with `@key`. A reconcile that asks a query, or sends a
  command again, adds a call too. So a test checks what a program sent without the program
  keeping a record of it:

  ```eye
  test "the morning's briefing goes at 07:40, for the Irish day" {
    let sent = fakes(notify, from: "recordings/")
    with sent
    with clock(2026-10-12 06:30 in "UTC")
    advance(10 min)
    let [briefing] = sent.calls(notify.send) else { unreachable("sent more or less than once") }
    expect(briefing.key == "ireland-2026-10-12" && briefing.priority == Normal)
  }
  ```

  `calls` takes an operation of a contract the fakes answer for; its result is a `List` of the
  record whose fields are the operation's parameters. `ai` and `http` have no operations, so
  their fakes show nothing this way. A call the fakes had no answer for stopped its run, so it
  isn't listed.
- **Services without a contract,** such as web pages, are read with the standard `http` module:
  `http.get!(url, headers: { "Accept": "text/html" }, via: proxy)` returns
  `Result(Response, HttpError)`, with the status, the headers and the body as text. Any answer is
  `Ok`, whatever its status; `Err` is `NoAnswer(reason)` or `TimedOut`. `via` is the URL of an HTTP
  proxy to go through. In tests, `with fakes(http, from: "recordings/")` answers `http.get` from
  `recordings/http.json`: `"contract": "http"`, and an exchange per URL, with `"op": "get"`, the
  URL as its `"key"` and `http.get`'s result (`{ "kind": "Ok", "value": { "status": 200, … } }`).
  An exchange without a key answers every other URL; a URL nothing answers fails the test.
- **Generated code for other languages** (types, clients and server stubs for Go, TypeScript, Rust
  and Python) is a build output, never committed.
