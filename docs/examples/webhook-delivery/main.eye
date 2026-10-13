# Webhook delivery. The shop tells a partner about each order event by
# calling the partner's webhook endpoint. Each event goes with a key, so the
# partner handles it once however often it arrives. A lost reply is settled
# by asking the partner. An endpoint that is down is tried again on a
# schedule this program writes for itself.

import endpoint
import http

## Something that happened that the partner wants to hear about.
pub type Event = { id: String, kind: String, body: String }

pub type Status =
  | Pending
  | Delivered(receipt: String)
  | GaveUp(why: String)

# region outgoing
## One event on its way to the partner.
entity Outgoing(id: String) {
  state status: Status = Pending

  ## Starts delivering an event, in one transaction. The run's key is the
  ## event's id, so queuing the same event again starts nothing.
  pub fn queue(e: Event) uses {start deliver} {
    deliver.start(e, key: "deliver-{id}")
  }

  pub fn finish(s: Status) { status = s }
}
# endregion outgoing

# region retry
## How long to wait after each failed try. The policy is this list: change
## it to change how often, and for how long, the program tries.
let backoff = [1 min, 5 min, 30 min, 2 h, 12 h]

## Whether another try could help: not after a success, and not after an
## answer that says no, such as 410 Gone.
fn worthRetrying(r: Result(a, http.HttpError)) -> Bool = match r {
  Ok(_) => false
  Err(BadStatus(code, _)) => code >= 500
  Err(_) => true
}

## Calls `send`, and calls it again after each wait in `waits` while the
## failure is worth retrying. Each sleep is a durable step, so a restart in
## the middle of a two-hour wait carries on where it was.
fn retry(waits: List(Duration), send: fn() -> Result(a, http.HttpError)) -> Result(a, http.HttpError) uses {wait} {
  var result = send()
  for d in waits {
    if !worthRetrying(result) { return result }
    sleep!(d)
    result = send()
  }
  result
}
# endregion retry

# region deliver
## Delivers one event. Every try sends the same key, so the partner acts on
## it once, even when an earlier try got through and only its reply was lost.
job deliver(e: Event) {
  let result = retry!(backoff, fn: endpoint.deliver!(e.kind, e.body, key: "{e.id}").reconcile!())
  let status = match result {
    Ok(ack) => Delivered(receipt: ack.id)
    Err(error) => GaveUp(why: "{error}")
  }
  Outgoing(e.id).finish!(status)
}
# endregion deliver

# ---------------------------------------------------------------- tests

let paid: Event = { id: "evt-1", kind: "order.paid", body: "\{\"order\": 1182\}" }

# region test-once
test "an event queued twice is delivered once" {
  let partner = fakes(endpoint, from: "recordings/")
  with partner
  Outgoing("evt-1").queue(paid)
  Outgoing("evt-1").queue(paid)
  expect(Outgoing("evt-1").status == Delivered(receipt: "wh-1"))
  expect(partner.calls(endpoint.deliver) |> size == 1)
}

test "a lost reply is settled by asking the partner, not by sending again" {
  let partner = fakes(endpoint, from: "recordings/")
  with partner
  Outgoing("evt-2").queue({ paid with id: "evt-2" })
  expect(Outgoing("evt-2").status == Delivered(receipt: "wh-2"))
  expect(partner.calls(endpoint.deliver) |> size == 1)
}

test "a lost reply the partner never got is sent again, with the same key" {
  let partner = fakes(endpoint, from: "recordings/")
  with partner
  Outgoing("evt-5").queue({ paid with id: "evt-5" })
  expect(Outgoing("evt-5").status == Delivered(receipt: "wh-5"))
  expect(partner.calls(endpoint.deliver) |> map(fn c: c.key) == ["evt-5", "evt-5"])
}
# endregion test-once

# region test-down
test "a partner that is down gets the event once it is back" {
  with clock(2026-10-12 09:00 in "UTC")
  with fakes(endpoint, from: "recordings/")  # three tries fail, the fourth gets through
  Outgoing("evt-4").queue({ paid with id: "evt-4" })
  advance(35 min)  # tries at 09:00, 09:01 and 09:06 failed
  expect(Outgoing("evt-4").status == Pending)
  advance(1 min)  # the try at 09:36 gets through
  expect(Outgoing("evt-4").status == Delivered(receipt: "wh-4"))
}

test "after the last wait, the delivery gives up and says why" {
  with fakes(endpoint, from: "recordings/down/")
  Outgoing("evt-1").queue(paid)
  advance(14 h + 35 min)
  expect(Outgoing("evt-1").status == Pending)
  advance(1 min)
  expect(Outgoing("evt-1").status == GaveUp(why: "BadStatus(code: 503, body: \"down for maintenance\")"))
}

test "an answer that says no is final" {
  let partner = fakes(endpoint, from: "recordings/")
  with partner
  Outgoing("evt-3").queue({ paid with id: "evt-3" })
  expect(Outgoing("evt-3").status == GaveUp(why: "BadStatus(code: 410, body: \"unsubscribed\")"))
  expect(partner.calls(endpoint.deliver) |> size == 1)
}
# endregion test-down
