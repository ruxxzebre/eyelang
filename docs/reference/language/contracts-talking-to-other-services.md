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
  command send(to: UserId, text: String, priority: Priority = Normal) -> Delivery

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
  contract without `@port` is an error at the contract's name. Callers never write a port in
  code: a call goes to the URL the configuration gives (below), and the network policy written
  from `@port` lets callers through to that port only, so in a deployment a call configured for
  another port reaches nothing. A contract without
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

  # trains/trains.eye
  import notify.{Identity, UserId}

  fn whoIs(user: UserId) -> Identity = notify.identity(user)    # error: trains isn't in @allow
  ```
- **`eye gen policy DIR` writes the network policy** that `@port` and `@allow` describe, as
  Kubernetes `NetworkPolicy` objects (`networking.k8s.io/v1`), one per contract with `@port`.
  Each module runs in pods labelled `eyelang.module: <module>`, its import path; the program's
  own module goes by the name of the program's directory.
  - The policy selects the pods of the module that serves the contract with `impl`. When nothing
    in the program serves it, as for a Notify written in TypeScript, it selects the pods labelled
    with the contract's own module, `eyelang.module: notify`, which that service's pods carry.
  - It lets in TCP on the contract's port from the pods of each module whose code calls one of
    its operations or names one as a value, among the modules `@allow` lets call it: the calls
    the checker refuses get no rule. Tests, a fake's `calls(op)` and the serving module itself
    don't count. With no caller, as under `@allow(nobody)`, it lets in no pod of the program;
    callers outside the program need a policy of their own, which Kubernetes adds to this one.
  - Contracts without `@port` get no policy, and egress isn't written: a service outside the
    network is reached as the cluster allows. Events, which the publisher posts to its
    subscribers' ports (below), get no rule yet (open question 48).
  - `--namespace homelab` puts the policies in that namespace; without it they name none, and
    `kubectl apply -n` decides. Each is named after the program and the contract (`lab-notify`
    for Notify in `testdata/lab`), so the policies of two programs that call one service add up
    rather than replace each other. Like generated code, the output is a build artifact.

  For the lab, whose `notifier` serves Notify:

  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: lab-notify
    namespace: homelab
  spec:
    podSelector:
      matchLabels:
        eyelang.module: notifier
    policyTypes:
      - Ingress
    ingress:
      - from:
          - podSelector:
              matchLabels:
                eyelang.module: access
          # … approving, digest and trains likewise
        ports:
          - protocol: TCP
            port: 3001
  ```
- **Commands** return a variant type or a record, and every command can also return
  `Uncertain(key)`: its reply was lost, so the other service may or may not have acted. Until it
  is reconciled, or a `match` handles `Uncertain(_)`, the result can't be passed where the
  declared type is expected, and a record's fields can't be read. Past an `Uncertain(_)` arm the
  reply wasn't lost: `match miniflux.createCategory!(title:) { Uncertain(_) => …, c => c.id }`. With `@reconcile(query)`,
  `.reconcile!()` resolves it by asking the query, which returns `Option` of the command's outcome:
  `Some` is the outcome, and `None` means the command never arrived, so it is sent again with the
  same key, which can't do it twice. A command without `@reconcile`, such as one to a service that
  takes no idempotency key, has no `.reconcile!()`: its caller matches `Uncertain(key)` and decides,
  usually by stopping for a person. Nothing is ever resent on a guess. A service that keeps what
  each key did, as Notify does, names the command itself: with `@reconcile(send)`, reconciling
  sends it again with the same key, and the service answers without doing it twice.
- **A command's outcome can't be dropped.** A statement whose value is of a type a command
  returns, with the `Uncertain`, `Stale` and `Expired` the command adds or without them once
  reconciled, must match it, use it, or discard it with `let _ = …`, as for a `Result`
  ([section 10](/reference/language/errors-result-only)). `notify.send!(owner, text, key: k)` alone is an error, since a `Refused` or an
  `Uncertain` reply would go nowhere; `let _ = notify.send!(…)` says that nothing depends on it.
- **Never sent is not uncertain.** A request that never reached the service (the connection was
  refused, or the name didn't resolve) can't have done anything, so it is sent again, as a query
  would be. Only a request that went out and whose reply was lost (a timeout, a broken connection,
  or a 5xx answer to a command) is `Uncertain(key)`.
- **Queries** are safe to repeat: one that can't reach its service is retried, while the run
  waits, and after a minute the run ends as `Failed(Unavailable(…))`. A query whose failures the
  caller handles declares them in its result: `query page(url: String) -> Result(String, FetchError)`,
  or, over HTTP, `Result(T, http.HttpError)`, which gets the failures instead of retrying.
- **One rule for waiting on a service:** the runtime tries again only where the program didn't
  ask to see the failure. An operation that declares `http.HttpError` asked, so it gets
  `Err(NoAnswer(…))` at once and waits its own way: the digest checks Miniflux's `me` every minute
  for up to six hours. An operation that declares no failures, and `http.get` and `ai.generate`,
  whose results always declare theirs so that a caller never chose, try again for about a minute
  what never reached the service (`http.get` below, `ai.generate` in [section 19](/reference/language/ai-models-and-agents)).
- **Events** are published with `notify.delivered.publish(d)`: from an entity method or an
  operation when the transaction commits, and from a job as a `!` step, `notify.delivered.publish!(d)`. `@key(field)` on an event
  names the field that tells one event from another, so an event delivered twice starts one run of
  each `@on(event(…))` job.
- **`@fresh`** on a command adds a required `basedOn: Version` argument and `Stale`/`Expired`
  outcomes, so an old command can't act on newer state.
  - `basedOn` comes after the parameters the command declares, and before `key`. It is the
    version of the state the caller read, as the service gave it; an entity's snapshot has one in
    its `version` field ([section 14](/reference/language/entities)), and an integer literal can be one.
  - `Stale` means the state changed after that version, and `Expired` that what the command was
    based on is gone, so it can never apply. Neither does anything. `Expired` is the same variant
    `approval!` gives.
  - Both join the variants of the command's result, so a `match` on it handles them, and until it
    does, the result can't be passed where the declared type is expected, as with `Uncertain`.
    Reconciling resolves `Uncertain` and keeps them.
  - A command whose result isn't a variant type, or already has a variant `Stale` or `Expired`,
    can't be `@fresh`.
- **`@destructive`** on a command adds a required `Approval` argument, `approval`, after the
  parameters it declares (and after `basedOn`), and before `key`. Only `approval!` makes an
  `Approval`, so a program calls the command only after a person said yes. On a function it adds
  the same parameter, which is how a model's tool asks for a person ([section 19](/reference/language/ai-models-and-agents)).

  ```eye
  # shelves/shelves.eye
  pub type Shelf = { id: String, items: List(String), version: Version }

  @port(3004)
  contract Shelves {
    query shelf(id: String) -> Shelf

    @fresh                                  # clear(id, basedOn)
    command clear(id: String) -> Cleared

    @destructive                            # burn(id, approval)
    command burn(id: String) -> Burned
  }

  # main.eye: reads again when the shelf changed in between
  job tidy(id: String) {
    let shelf = shelves.shelf!(id)
    match shelves.clear!(id, basedOn: shelf.version) {
      Cleared(n) => Log(id).add!("cleared {n}")
      Stale => {
        let _ = tidy.start!(id)
      }
      Expired | Uncertain(_) => ()
    }
  }

  # a program that serves Shelves answers from its entity's snapshot
  impl shelves.Shelves {
    fn clear(id, basedOn) {
      let rack = Rack(id).read()
      if rack.gone { return Expired }
      if rack.version != basedOn { return Stale }
      Cleared(count: Rack(id).empty())
    }

    fn shelf(id) {
      let rack = Rack(id).read()
      { id, items: rack.items, version: rack.version }
    }

    fn burn(id, approval) = Burned(count: Rack(id).burn())
  }
  ```
- **A contract's operations are its module's functions.** A module that declares one contract offers
  its commands and queries by name: `notify.send!(…)` for `contract Notify { command send … }`.
- **Serving a contract.** A program written in Eyelang serves a contract with `impl Contract { … }`,
  in any of its modules ([section 11](/reference/language/traits)): one function per operation, named after it, with its
  parameters in order and `key` last for a
  command with `@key`. Types and defaults come from the contract. Each runs as one transaction, as
  an entity method does, so it can't wait or reach outside the program ([section 14](/reference/language/entities)), and it may
  start jobs and publish events. A key can name an entity, so a
  repeated command finds the work it already did:

  ```eye
  impl notify.Notify {
    fn send(to, text, priority, key) = Message(key).queue({ text, to, priority })
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
  - A `Float` is a number. NaN and the infinities have no JSON form, so writing a value that holds
    one is a fault at the call that writes it (`toJson`, a request, a reply, a tool's result),
    which ends the run as `Unreachable` ([section 15](/reference/language/jobs-and-waiting)) with a message that names the value: `toJson:
    NaN can't be written as JSON, which has no NaN or infinities`. It isn't `null`, as some
    encoders write, because `null` is `None`: `Some(nan)` would read back as `None`.
  - An `Instant` is an RFC 3339 string, a `Date` `"2026-10-12"`, a `Duration` a number of
    milliseconds, and a `Size` a number of bytes.
  - A `Version` is a number or a string, as the service gave it; a snapshot's is a number. An
    `Approval` is `{ "what": "…", "by": "…" }`.
  - The outcomes a contract adds to a command are objects tagged with `"kind"`, whatever the
    shape of the command's own variants: `{ "kind": "Stale" }`, `{ "kind": "Expired" }` and
    `{ "kind": "Uncertain", "key": "…" }`. A reader takes `"Stale"` and `"Expired"` alone too.

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
    command send(to: UserId, text: String, priority: Priority = Normal) -> Delivery
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
  - A `@fresh` command sends `basedOn` and a `@destructive` one `approval` as arguments, like the
    others (`based_on` with `names: snake`). A reply of 409 to a `@fresh` command is `Stale` and
    one of 410 is `Expired`, unless `status:` maps those codes; `status: { 412: Stale }` maps
    another. The service can't check an approval, only record it, so `@allow` is what limits who
    may send one.
  - `@auth(bearer: "secret")`, or `@auth(header: "X-Auth-Token", secret: "secret")`, sends a
    secret with every request.
  - A refused request (a 4xx that `status:` doesn't name) ends the run with a fault that shows the
    reply, `Unreachable` ([section 15](/reference/language/jobs-and-waiting)): it is a bug in the program or its configuration, not an
    outcome to handle.

  A program finds a contract's service at the URL in the environment variable named after its
  module (`NOTIFY_URL` for `notify`, `LAB_NOTIFY_URL` for `lab.notify`). A program that serves a
  contract itself calls its own implementation, unless that variable is set. The URL alone
  decides where a call goes, port included; `@port`, and `NOTIFY_PORT` below, decide only where a
  program that serves the contract listens. A server moved with `NOTIFY_PORT`, as in a test, is
  reached by callers whose `NOTIFY_URL` names that port.
- **Serving over HTTP.** `eye run` serves each contract the program serves on the contract's
  port, on every network interface, and runs until it is stopped. It names the port when it
  starts: `eye run: serving notify.Notify on :3001`. `NOTIFY_PORT`, named after the module as
  `NOTIFY_URL` is, serves it on another port instead, and `0` on any free one, so tests can run a
  server and its callers side by side. A request is read the way a caller sends it, so both sides
  follow the same attributes:
  - The operation's `@http` method and path pick it. An operation without `@http` is reached only
    from inside the program. A path nothing answers gets 404, and a known path with another method
    405.
  - Arguments come from the path, the query parameters or the JSON body, in the shape above. One
    left out takes its default, an `Option` left out is `None`, and anything else missing or of the
    wrong shape gets 400. A command with `@key` reads its key from the key's header, or from its
    body `field:`, and gets 400 without one. A refusal's body is `{ "error": "…" }`.
  - With `@auth`, a request without the secret, as a caller sends it, gets 401 and reaches no
    code. `eye run` reads the secret when it starts, and doesn't start without it.
  - The reply is the result in the shape above. A result whose variant `status:` names has that
    status, the lowest when several name it (Notify's `Refused` is 400); any other result has 200.
  - Each request runs its operation as one transaction, and the jobs it starts run after it
    commits, among the program's other runs. A fault answers 500 and changes nothing, so the
    caller's command is `Uncertain` and reconciles. The runtime keeps no replies: a repeated key
    reaches the `impl` again, which finds the work it already did through an entity the key
    names, as Notify's `Message(key)` does.
  - Requests are answered one at a time, between the program's steps and while its runs wait for
    a timer or another service.
- **Events between programs.** A contract's events go from the program that serves the contract
  to the programs that wait for them, as HTTP requests that follow the contract's attributes, the
  way its operations do. Notify's `answered`, `on "notify.answers"`, reaches a program as:

  ```text
  POST /events/notify.answers
  Authorization: Bearer <secret("notify-token")>

  { "question": "q-1", "choice": "Approve", "by": "paul", "at": "2026-10-12T09:15:00Z" }

  → 202 Accepted
  ```

  - **Taking events.** `eye run` takes each event the program consumes (an `@on(event(…))` job
    or a `next!` on it, the `consume` effect) of a contract it doesn't serve, as
    `POST /events/<topic>`. The body is the event's value in the shape above, with the contract's
    `@http(names:)`.
  - Only the events of a contract with `@auth` are taken, and a request without the contract's
    secret gets 401 and reaches no code: the publisher sends the secret its callers send it. A
    contract without `@auth` would let anyone who reaches the port publish its events, such as
    a button press that grants an approval, so `eye run` says when it starts that they have no
    way in: `eye run: telegram.updates has no way in: contract Telegram has no @auth, so anyone
    could publish it`.
  - A body that isn't of the event's type gets 400, a topic the program doesn't take 404, and
    another method 405, each with `{ "error": "…" }`.
  - An event taken is published in the program as one of its own would be: it starts each
    `@on(event(…))` job and wakes the runs waiting in `next!` for one like it. The answer, 202
    with no body, comes once the event is in the journal, so a publisher that has it may forget
    it. A repeat of an event with `@key` gets 202 too, and starts nothing.
  - The events of a contract the program serves aren't taken, since only its `impl` publishes
    them, nor those of the standard library's contracts: `eye approve` answers approvals.
  - **Where.** Events are taken on each port the program serves a contract on, and on
    `EYE_EVENTS_PORT` when it is set, `0` for any free one, which a program that serves nothing
    needs. `eye run` names each port when it starts: `eye run: taking notify.answered on :8080`.
    A program that consumes another service's events with neither says so when it starts, since
    nothing can reach it: `eye run: notify.answered has no way in: set EYE_EVENTS_PORT to the port
    its publisher sends it to`. A program that takes events keeps running until it is stopped, as
    one that serves a contract does. An operation served on the same port at the same
    `POST /events/<topic>` stops `eye run` before it listens.
  - **Sending events.** A program that publishes an event of a contract it serves sends it to
    each subscriber that `NOTIFY_EVENTS_URL` lists, named after the module as `NOTIFY_URL` is:
    base URLs separated by commas or spaces, such as `http://approving:8080,http://trains:8080`.
    Each gets `POST <url>/events/<topic>` with the event's value and the contract's `@auth`.
    Without the variable, the events stay in the program.
  - The event and a delivery for each subscriber are saved in the commit that publishes it, and
    sent after it. A delivery is done when the subscriber answers with a success. With no answer,
    or a 5xx, it is sent again, by the one rule for a failure the program didn't ask to see: after
    half a second, doubling to every 30 seconds, across restarts, since the journal keeps it. A
    4xx other than 408 and 429 is a refusal, such as a wrong URL or secret: a bug in the
    configuration, so the delivery is dropped with a line in the log that names the subscriber
    and its reply. A delivery not taken within 30 days, as long as a subscriber remembers an
    event's key, is dropped the same way.
  - Each subscriber gets the events in the order they were published: a delivery waits until the
    one before it to the same subscriber is done or dropped.
  - So an event arrives at least once, and one with `@key` starts its jobs once: a delivery whose
    answer was lost is sent again, and the subscriber knows the key. An event without `@key` that
    is sent again starts its jobs again, so give an event that starts work a key.
  - `eye test` sends and takes nothing: a test publishes events with `c.e.publish(…)`.
- **Fakes come from recordings.** `with fakes(notify, from: "receipts/")` replays real recorded
  exchanges, checked against the contract. Reading a field that no recording contains is a compile
  error, unless it is marked `@unwitnessed("why")`:
  - The recordings are the ones the program's tests replay: each `fakes(m, from: "dir/")` whose
    directory is written as a string names `dir/m.json`, beside the file it is written in. A
    contract none of them records isn't checked, and nor is one the program serves.
  - A field is one of a type the contract's module declares and an operation's result holds. A
    recording contains it when some recorded result has it, present even if `null`.
  - A read is a field access, `r.receipt`, or a pattern that looks inside the field,
    `Refused(error)`. `Refused(_)` and `Refused(..)` read nothing, so a match handles a variant
    no recording has without a mark.
  - `@unwitnessed("why")` goes on the function, job or test that reads the field, or on the type,
    and says why the author believes the service sends what no recording shows.

  A recording is a JSON file per contract module,
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
- **A test sees what a fake received.** `fakes(…)` is a value of the type `Fakes`: a test that
  wants to look names it, applies it with `with`, and asks it with `calls(op)`. Only a `Fakes`
  has `calls`: `clock(…)` is a plain `Handler`, so `c.calls(op)` on it is a check error. That lists every call of the operation
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
  proxy to go through. A request that never reached the server (the connection was refused, or the
  name didn't resolve) is sent again for about a minute, as a query's is, and then is
  `NoAnswer(reason)`, the reason in words: "www.rte.ie refused the connection, 8 times over a
  minute". A request that reached it and had no answer in time is `TimedOut` at once. In tests, `with fakes(http, from: "recordings/")` answers `http.get` from
  `recordings/http.json`: `"contract": "http"`, and an exchange per URL, with `"op": "get"`, the
  URL as its `"key"` and `http.get`'s result (`{ "kind": "Ok", "value": { "status": 200, … } }`).
  An exchange without a key answers every other URL; a URL nothing answers fails the test.
- **Generated code for other languages.** `eye gen go DIR` and `eye gen ts DIR` write, for each
  module that declares contracts, its types, a client and a server stub: a Go package, or a
  TypeScript module. Like the network policy, the output is a build artifact, never committed.
  Rust and Python may follow. Both read the same rules as the interpreter, so a generated client,
  a generated stub and `eye run` send and read the same requests.

  ```sh
  eye gen ts --module notify testdata/lab > notify.ts
  eye gen go --out gen testdata/lab   # gen/notify/notify.go, gen/miniflux/miniflux.go, …
  ```

  - The types are the one JSON shape above, with their `##` comments. TypeScript's describe the
    JSON itself: a record is an interface with the names on the wire (`published_at`), a variant
    type is a union discriminated on `"kind"`, a type of fieldless variants a union of strings, a
    wrapper its value's type, and `Option(T)` is `T | null`. Go's marshal to it: a struct for each
    record and each variant, an interface for a variant type (`Delivery`, which `Accepted`,
    `Pending` and `Refused` implement), a string type with constants for fieldless variants, `*T`
    for an `Option`, `time.Time` for an `Instant`, and `Duration`, in milliseconds on the wire.
    Go names take Go's capitals: `MessageId` is `MessageID`.

    ```ts
    export type Delivery = Accepted | Pending | Refused;

    export interface Accepted {
      kind: "Accepted";
      id: MessageId;
      deliveries: number;
      deduped: boolean;
    }
    ```

  - Every type the contracts' operations and events use is written, from whichever module
    declares it. When two things would have one name, both take a longer one: a type puts its
    module's name in front, and a variant its type's, as NetBird's record `Pending` and Notify's
    variant `Pending` become `NetbirdPending` and `DeliveryPending`. A variant type with one
    variant of its own name is that variant alone.
  - A client calls each operation that has `@http` as the interpreter does: path, query or body,
    defaults filled in, a `None` left out, `@auth`, the key's header and body field, and
    `status:`. Its answers are the operation's, with what the contract adds: in TypeScript,
    `Uncertain` for a lost reply, and `Stale` and `Expired`, as values; in Go, as the errors
    `*Uncertain`, `ErrStale` and `ErrExpired`. A query of `Result(T, http.HttpError)` returns
    `T`, and an answer that isn't a success is an `HttpError` (Go's `*HTTPError`). A request
    that never reached the service gives the error that stopped it, never `Uncertain`, so it can
    be sent again.
  - A server stub is an interface to implement, `NotifyService`, and a handler that serves it
    as `eye run` serves the contract: Go's `NewNotifyHandler(service, secret)` is an
    `http.Handler`, and TypeScript's `notifyHandler(service, secret)` a function from a Fetch API
    `Request` to a `Response`. It checks `@auth` first, routes by `@http`, reads the arguments,
    gives those left out their defaults, reads the key, and answers with the status `status:`
    gives the result's variant. A method that fails answers 500, which the caller's command takes
    as `Uncertain`; returning `ErrStale` or `ErrExpired` in Go, or `Stale` or `Expired` in
    TypeScript, gives those outcomes. A Go client implements its service interface, so a server
    can pass calls on.
  - Each event gets a sender, for a service in that language that publishes it: Go's
    `SendAnswered(ctx, client, baseURL, secret, ev)` and TypeScript's
    `sendAnswered(options, event)` send it to one program that takes it, as `eye run` sends it
    (`POST /events/<topic>` with `@auth`). They return once the program has it, and give an
    error otherwise, an `HttpError` for a refusal; keeping the event and sending it again until
    it is taken, as `eye run` does, is the service's part.
  - Operations without `@http` have no client method and no route, as `eye run` gives them none.
  - A type these languages' JSON can't say the same way, a tuple, a map whose keys aren't strings
    on the wire, an unnamed record or an `extern type`, is an error that names it.
