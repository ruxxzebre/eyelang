# What SPECS.md §15's jobs assume (context=notify,jobs): the feed reader
# and Notify they call, who they tell or ask, how a digest is written, and
# what deploying a build does.

import fs
import notify.{Delivery, UserId, Choice}
import miniflux

type DigestError =
  | FeedFailed(miniflux.FeedError)
  | Faulted(Fault)

type Article = { title: String, text: String }

let owner = UserId("operator")

let operator = UserId("operator")

let yes: Choice = { id: "yes", label: "Yes" }

let no: Choice = { id: "no", label: "No" }

## Writes the day's summary; it reads the clock, so a job calls it as a step.
fn summarise(items: List(miniflux.Item)) -> String uses {clock} =
  "{size(items)} items by {now()}"

## Writes the day's article.
job write(day: Date) -> Result(Article, DigestError) {
  let items = miniflux.unread!("ireland", since: day - 1 day)?
  Ok({ title: "Ireland, {day}", text: summarise!(items) })
}

## Ships a build that a person approved, and notes who approved it.
fn deploy(build: String, ok: Approval) -> Result((), fs.FsError) uses {fs} =
  fs.appendText("deploys.log", "{build}, approved by {ok.by}\n")
