# The lab in Eyelang

The stateful, waiting workloads of the homelab (the `stacks-homelab`
repository), written in Eyelang to test the language on real programs. Each
module names the code it is modelled on.

| Module | What it is | Modelled on |
|---|---|---|
| `digest/` | The Ireland digest: every morning at 07:40 Irish time, read the news, have a model write a briefing, send it | `js/apps/automation/src/digest` (Hatchet, TypeScript) |
| `notifier/` | Notify itself: an outbox per person with quiet hours, and questions whose first answer wins | `js/apps/notify` (TypeScript, PostgreSQL outbox) |
| `trains/` | Train reminders with "Boarded" and "Not taking it" buttons, and delay alerts | `services/applications/homebrew/internal/apps/trains` (Go) |
| `access/` | NetBird access: a sync every minute that stops on a doubtful step, and admission of new users | `services/infrastructure/netbird-sync` (Go) |
| `home/` | What they share: people, the operator, the lab's time zone | |
| `notify/`, `telegram/`, `miniflux/`, `flaresolverr/`, `irishrail/`, `netbird/`, `authentik/` | Contracts: the services the programs call. Notify, Miniflux and FlareSolverr say how they go over HTTP, matching the lab's real APIs | |
| `web/` | Pages without a feed, read with `http.get`, directly or through the VPN's proxy, or with FlareSolverr | `js/apps/automation/src/digest/http.ts` |

`go test ./interp -run TestLab` checks the program and runs its tests, which
replay recorded exchanges with the services (`recordings/` next to each
module). `eye test testdata/lab` does the same from the command line. The
digest's morning runs whole there, its pages and model call included, on
recordings of Miniflux, FlareSolverr, `http.get`, the model and Notify.
`go test ./interp -run TestDigestOverHTTP` runs the morning digest against
stand-ins for Miniflux, the VPN's proxy, FlareSolverr, the model gateway and
Notify, over HTTP.

Run for real, the program reads its services' addresses and secrets from the
environment:

| Variable | What it is |
|---|---|
| `MINIFLUX_URL`, `MINIFLUX_KEY` | Miniflux, and its API key (`X-Auth-Token`) |
| `NOTIFY_URL`, `NOTIFY_TOKEN` | Notify, and the digest's bearer token |
| `FLARESOLVERR_URL`, `VPN_PROXY_URL` | FlareSolverr, and the VPN's HTTP proxy |
| `AI_URL`, `AI_KEY`, `DIGEST_MODEL` | The OpenAI-compatible model gateway (CLIProxyAPI), its key, and the model: `gpt-5.6-terra` when `DIGEST_MODEL` isn't set |
| `AI_PRICES` | Prices of models the runtime doesn't know, in dollars per million tokens in and out: `gpt-5.6-terra=IN/OUT`. Required for the digest |

The digest's `write` job has `@budget(usd: 0.50)`. The runtime knows the
prices of Anthropic's models only, so a run on any other model needs that
model's price in `AI_PRICES`. Without it, `write` fails before the model is
asked: the run ends with a fault that names the model and `AI_PRICES`, and
the morning job sends "Ireland digest not sent" instead of the briefing. Give
the price the provider lists for the model, even when the gateway bills by
subscription, since the budget is there to stop a runaway prompt. A gateway
that names the model with its provider (`openai/gpt-5.6-terra`) is matched by
the part after the slash.

Secrets (`MINIFLUX_KEY`, `NOTIFY_TOKEN`, `AI_KEY`) can also be files named
`miniflux-key`, `notify-token` and `ai-key` in the directory `$EYE_SECRETS`,
which is how Kubernetes mounts them. A variable, when set, wins over a file.

## Deploying the digest

This runs the morning digest in the homelab's Kubernetes cluster on the
Postgres store. Notify stays the TypeScript app; the digest calls it over
HTTP, as the TypeScript digest does.

### What runs

`eye run` runs every job of every module a program imports, so it never runs
`testdata/lab` itself: that would start the notifier, the train reminders and
the NetBird sync, which act on the real Telegram, trains and NetBird.
`deploy/digest/` is the digest alone: a program whose own module only imports
`digest`, and whose module directories (`digest`, `home`, `notify`,
`telegram`, `miniflux`, `web`, `flaresolverr`) are links to the lab's.
`go test ./interp -run TestDigestDeployProgram` checks that it has the
digest's jobs and no others, and runs its tests.

Its jobs:

- `digest.morning`, at 07:40 Irish time, starts `digest.write` (read Miniflux
  and two pages, ask the model; at most $0.50 and 7 hours) and sends the
  briefing to Notify's `/v1/send` with the key `ireland-YYYY-MM-DD`, or a
  failure notice with `ireland-YYYY-MM-DD-failed`.
- `digest.command` answers `/digest` from Telegram updates. Nothing in this
  program receives Telegram updates, so it never runs: `/digest` stays with
  the TypeScript automation. (It would also need Notify's identity check,
  which the contract doesn't say how to reach over HTTP yet.)

A schedule counts from the program's first start: started after 07:40, the
first digest is the next morning's.

### The image

1. Build `eye` from the same eyelang commit as the program, for Linux:

   ```sh
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o eye ./cmd/eye
   ```

   This needs Go 1.25. The binary is static and carries its time zones, so a
   `distroless/static` or `scratch` base is enough.
2. Copy the program, following its links:

   ```sh
   cp -RL testdata/lab/deploy/digest /program
   ```

   A Dockerfile's `COPY` keeps links as links, so copy with `cp -RL` into the
   build context first. The image holds `/eye` (or `/usr/local/bin/eye`) and
   `/program`, the `.eye` sources and the recordings; nothing else.
3. Check it in the build: `eye check /program && eye test /program`.

### The command

```sh
eye run --memory-limit 384MiB /program
```

with `EYE_STORE` set to the database (below), or `--store postgres://…`
instead of `EYE_STORE`. It runs until SIGTERM, which stops it cleanly; a
restart carries on where it stopped, and a run killed in the middle of a step
does that step again. `--memory-limit` is needed in a container: without it
the runner takes the node's memory from `/proc/meminfo`, not the pod's limit.
It must be at least the write job's `@memory(256 MiB)`, or the write run
fails as `OutOfMemory`; give the pod a limit above it, such as 512Mi.

It logs one line when it starts, on stderr with the rest of its log, so a
healthy pod's log is never empty: the program, the store (host, database
and schema, never the user or password), the memory runs may hold, and the
jobs, with when each scheduled one fires next:

```
eye run: started /program; store: postgres postgres-rw.lab.svc.cluster.local:5432/app schema lab; memory: 384 MiB; jobs: digest.command, digest.morning (next 2026-10-12 07:40 Europe/Dublin), digest.write
```

Run one replica, with `strategy: Recreate`: the store allows one `eye run` per
database and schema, through an advisory lock, and a second one exits with
"… is in use by another eye run".

### The store

The store is CloudNativePG's database for the app. `EYE_STORE` is the `uri`
from the app's secret (`postgres-app`), with the schema added:

```sh
EYE_STORE=postgresql://app:…@postgres-rw.NAMESPACE.svc.cluster.local:5432/app?search_path=lab
```

For example, in a database `digest` with the schema `eyelang`:
`postgresql://…/digest?search_path=eyelang`, after `CREATE SCHEMA eyelang;`.

- `postgres://` and `postgresql://` are both taken. Any libpq parameter
  works beside `search_path` (`sslmode=require`, `connect_timeout=5`; pgx
  reads them). Without `sslmode`, the connection tries TLS and falls back to
  plain, which CloudNativePG's default TLS satisfies. In a pod spec, build the
  variable from the secret with `$(DATABASE_URL)?search_path=lab`, after
  `DATABASE_URL` is defined from the secret's `uri`.
- The tables (`eye_store`, `eye_rows`, `eye_runs`, `eye_events`, `eye_meta`)
  are made on the first start, in the first schema of the search path. The
  schema is not made: create it once, as the app user (who owns the database
  and so may), `CREATE SCHEMA lab;`, or leave out `search_path` and use
  `public`, which the database owner owns on Postgres 15 and later.
- The user needs to create tables in that schema and read and write them,
  and calls `pg_try_advisory_lock`, which every user may. The app user has
  all of this; nothing needs a superuser.
- A database that restarts or goes away doesn't stop the runner: a commit
  that can't be written is tried again, and `eye run` says "trying again".
- Nor does a database that can't be reached yet when `eye run` starts, as
  while a network policy is still landing: it says "trying again" every 10
  seconds until it connects, or until SIGTERM. It waits when the connection
  is refused, reset or times out, when the host's name doesn't resolve,
  and when Postgres says it is starting, shutting down or out of
  connections (SQLSTATE `57P01`–`57P03`, `53300`, class `08`). It exits 1 at
  once on what waiting won't fix: an address it can't read, a wrong user or
  password, refused TLS, a database or schema that doesn't exist, a user
  that may not make the tables, a schema another `eye run` holds, or tables
  a newer `eye` wrote.

### The filesystem

On the Postgres store `eye run` writes nothing to disk: no journal, cache,
lock file or temporary file, and nothing in `$HOME` (`go test ./interp -run
TestRunOnAStoreWritesNoFiles`). It reads `/program`, the secrets, and
`/program/.eye/inbox` if it exists, for `eye approve`'s answers, which the
digest doesn't use. A read-only root filesystem without a writable `/tmp` is
fine.

### The environment

| Variable | Value in the cluster |
|---|---|
| `EYE_STORE` | `$(DATABASE_URL)?search_path=lab`, as above |
| `EYE_SECRETS` | `/secrets`, the mounted secrets below |
| `MINIFLUX_URL` | `http://miniflux.miniflux.svc.cluster.local:8080` |
| `FLARESOLVERR_URL` | `http://flaresolverr.flaresolverr.svc.cluster.local:8191`, without the `/v1` the TypeScript digest's address has: the contract adds it |
| `VPN_PROXY_URL` | `http://vpn-proxy.vpn-proxy.svc.cluster.local:8888` |
| `AI_URL` | `http://cliproxyapi.cliproxyapi.svc.cluster.local:8317/v1`, the gateway's base: the runtime adds `/chat/completions` |
| `NOTIFY_URL` | `http://notify.messaging.svc.cluster.local:8080`, where `/v1/send` is |
| `DIGEST_MODEL` | optional; `gpt-5.6-terra` without it |
| `AI_PRICES` | `gpt-5.6-terra=IN/OUT`, the model's price per million tokens: required (see above) |
| `DIGEST_EVENT` | optional; `digest.ireland` without it. See "Running beside the TypeScript digest" |
| `DIGEST_TO` | optional; Authentik uids separated by commas, at most 20, that the briefing and its failure notice go to instead of everyone who receives the event. See "Running beside the TypeScript digest" |

The addresses are the ones the TypeScript digest's deployment
(`clusters/mainframe/infrastructure/automation/deployment.yaml`) uses today.

Secrets, as files in `$EYE_SECRETS`, one value each (a trailing newline is
dropped):

| File | What it holds | From the secret the TypeScript digest mounts |
|---|---|---|
| `miniflux-key` | Miniflux's API key | `digest-miniflux`, key `api-key` |
| `notify-token` | Notify's bearer token for the digest (`TOKEN_DIGEST`, service `digest`) | `notify-digest-token`, key `TOKEN_DIGEST` |
| `ai-key` | The gateway's key | `inbox-suggest`, key `token` (mounted as `gateway-key` today; the name here must be `ai-key`) |

The pod needs to reach Miniflux, FlareSolverr, the VPN's proxy, CLIProxyAPI,
Notify's port 8080 and Postgres; the write job reads no page directly, only
through the proxy and FlareSolverr.

### Running beside the TypeScript digest

While the TypeScript digest still sends, the two can run side by side, with
the Eyelang digest going to the operator alone:

```sh
DIGEST_EVENT=digest.ireland.eye
DIGEST_TO=<the operator's Authentik uid>
```

- `DIGEST_TO` narrows the send to the uids it lists, separated by commas
  (Notify's `to`, at most 20; spaces and empty items are ignored). Each
  still needs the event's category and their own settings to accept it, so
  subscribe the operator to `digest.ireland.eye` in Notify. More than 20 fails
  the morning's run with a fault that names `DIGEST_TO`.
- `DIGEST_EVENT` sends on an event of its own, so the operator can tell the
  two digests apart and mute one. `TOKEN_DIGEST` may send any `digest.*`
  event. Without `DIGEST_TO`, everyone subscribed to `digest.ireland.eye`,
  `digest.*` or `*` gets it.
- With either set, the keys start with the event
  (`digest.ireland.eye:ireland-YYYY-MM-DD`). Notify drops a key it has seen
  from the same caller in the last six hours, whatever its event and
  recipients, and both digests send as `digest` with `ireland-YYYY-MM-DD`:
  with the same keys, whichever sent second would be dropped, and readers
  could lose the TypeScript digest to the operator's copy.

The failure notice goes the same way. To switch over, stop the TypeScript
digest's schedule and remove both variables. Notify then drops the
morning's second send by its key, if both send that day.

### After the first morning

At 07:40 Irish time, the run starts. From the pod:

```sh
eye runs /program            # every run: job, status, result or fault
eye runs /program 2          # one run's steps, by its number
```

`eye runs` reads `EYE_STORE` too, without changing anything, while `eye run`
runs. A good morning shows:

```
#1 digest.morning(2026-10-12)  done: Ok(Accepted(id: "m-…", deliveries: 1, deduped: false))  [7 steps, 1.4 KiB]
#2 digest.write(2026-10-12)  done: Ok({ day: 2026-10-12, title: "Ireland & Dublin — Monday 12 October", document: …  [10 steps, 1.8 KiB]
```

`eye runs /program 2` lists the write run's steps: `miniflux.me`, the pages
(`http.get`, `flaresolverr.request`; a page that couldn't be read shows
`Err(…)` and is listed under "Sources not read" in the briefing), three
`miniflux.entries`, `ai.generate` and `Day(…).keep`. It doesn't show what
the model call spent yet. What a bad morning shows, and what to do:

| `eye runs` shows | Meaning | Do |
|---|---|---|
| write `failed: … the price of gpt-5.6-terra isn't known …`, morning `done: Err(Faulted(…))` | `AI_PRICES` is missing | Set it; the next morning's run uses it |
| write `failed: … set MINIFLUX_URL …` (or another service's) | A variable is missing | Set it, the same way |
| morning `done: Err(NoReaders)` | Notify queued it for nobody | Subscribe someone to the event in Notify |
| morning `done: Err(NotSent(…))` | Notify refused it | Check the token and that the event is in its scope |
| morning `done: Err(MinifluxAway)` | Miniflux stayed paused for six hours | Nothing: readers got "Ireland digest not sent" |
| morning `sleeping` | Waiting for the write run, which waits for Miniflux while it is paused | Nothing |

Each failed morning also sends "Ireland digest not sent" on the event, with
the reason.

`eye run`'s log (the pod's output) also says when a run fails, and "trying
again" while the database is away.
