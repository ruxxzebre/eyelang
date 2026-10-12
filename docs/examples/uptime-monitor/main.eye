# Uptime monitor. Every minute each watched site gets an HTTP GET. Two failed
# probes in a row mean the site is down. The team hears once when it goes
# down and once when it is back, with how long it was out.

import chat
import http

## Where alerts go.
let channel = "#ops"

# region probe
## What one probe found.
pub type Probe =
  | Up
  | Down(why: String)

## Reads a probe's answer. A 5xx, or no answer at all, is down.
pub fn read(answer: Result(http.Response, http.HttpError)) -> Probe = match answer {
  Ok(page) => if page.status < 500 then Up else Down(why: "it answered {page.status}")
  Err(error) => Down(why: "{error}")
}
# endregion probe

# region site
## A site to watch, and what its recent probes found.
entity Site(name: String) {
  state url: String = ""
  ## Failed probes in a row.
  state failures: Int = 0
  state why: String = ""
  state failingSince: Option(Instant) = None
  state outages: Int = 0
  state lastOutage: Duration = 0 s

  pub fn watch(address: String) { url = address }

  ## Keeps what one probe found.
  pub fn record(p: Probe, at: Instant) {
    match p {
      Down(reason) => {
        if failures == 0 { failingSince = Some(at) }
        failures += 1
        why = reason
        if failures == 2 { outages += 1 }
      }
      Up => {
        if failures >= 2 { lastOutage = at - (failingSince ?? at) }
        failures = 0
        failingSince = None
      }
    }
  }
}
# endregion site

# region monitor
## Probes every site once a minute. With @concurrency(1), a round that runs
## long makes the next minute's round skip, rather than pile up behind it.
@on(every(1 min))
@concurrency(1)
job probe() {
  for site in Site.all.read!() {
    let found = read(http.get!(site.url))
    Site(site.name).record!(found, now!())
  }
}

## Every site that failed two probes in a row and hasn't answered since.
view down = Site.all |> filter(fn s: s.failures >= 2)

## A site that enters the view has just gone down. The view says when, so
## there is no "alerted" flag to keep: the run starts once per outage.
@on(enters(down))
job wentDown(s: Ref(Site)) {
  let site = s.read!()
  tell!("{site.name} is down: {site.why}", key: "down-{site.name}-{site.outages}")
}

## A site that leaves the view is back.
@on(leaves(down))
job cameBack(s: Ref(Site)) {
  let site = s.read!()
  tell!("{site.name} is back after {site.lastOutage}", key: "up-{site.name}-{site.outages}")
}
# endregion monitor

@unwitnessed("the chat refuses only an archived channel, which no recording has")
fn tell(text: String, key: Key) {
  match chat.post!(channel, text, key:).reconcile!() {
    Posted(_) => ()
    Refused(error) => log.warn("the chat refused {key}: {error}")
  }
}

## Watches the sites named on the command line, for example
## `eye run . -- shop=https://shop.example.com api=https://api.example.com`.
pub fn main(args: List(String)) uses {entity Site} {
  for arg in args |> drop(1) {
    let [name, url] = arg |> split("=") else { continue }
    Site(name).watch(url)
  }
}

# ---------------------------------------------------------------- tests

fn posted(f: Fakes) -> List(String) = f.calls(chat.post) |> map(fn c: c.text)

# region test-outage
test "an outage is told once when it starts, and once when it ends" {
  let team = fakes(chat, from: "recordings/")
  with team
  with clock(2026-10-12 09:00 in "UTC")
  with fakes(http, from: "recordings/up/")
  Site("shop").watch("https://shop.example.com")
  advance(5 min)
  with fakes(http, from: "recordings/down/")
  advance(1 min)  # one failed probe could be a blip
  expect(posted(team) == [])
  advance(10 min)  # ten more, one message
  expect(posted(team) == ["shop is down: it answered 503"])
  with fakes(http, from: "recordings/up/")
  advance(1 min)
  expect(posted(team) == ["shop is down: it answered 503", "shop is back after 11 min"])
}
# endregion test-outage

test "a single failed probe is not an outage" {
  let team = fakes(chat, from: "recordings/")
  with team
  with fakes(http, from: "recordings/down/")
  Site("shop").watch("https://shop.example.com")
  advance(1 min)
  with fakes(http, from: "recordings/up/")
  advance(1 h)
  expect(posted(team) == [])
}

test "a probe's answer is read as up or down" {
  expect(read(Ok({ status: 200, headers: {}, body: "" })) == Up)
  expect(read(Ok({ status: 404, headers: {}, body: "" })) == Up)
  expect(read(Err(TimedOut)) == Down(why: "TimedOut"))
}
