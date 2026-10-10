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
| `AI_URL`, `AI_KEY`, `DIGEST_MODEL` | The OpenAI-compatible model gateway (CLIProxyAPI), its key, and the model |
