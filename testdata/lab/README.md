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
module). `eye test testdata/lab` does the same from the command line.
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
which is how Kubernetes mounts them.
