import miniflux
import notify

let owner = UserId("operator")

@on(daily(07:40, in: "Europe/Dublin", onGap: shift, onOverlap: first))
@priority(background)
@memory(64 MiB)
@deadline(13:40, in: "Europe/Dublin")
@concurrency(1)
job irelandDigest(day: Date) -> Result(Delivery, DigestError) {
  let items = miniflux.unread!("ireland", since: day - 1 day)?
  if items == [] {
    return Err(NothingNew)
  }
  let text = reserve!(256 MiB, fn {
    summarise!(items)
  })
  Ok(notify.send!(owner, text, key: "ireland-{day}").reconcile!())
}

pub fn summarise(items: List(Item)) -> String uses {wait, net Llm.complete} {
  llm.complete!(prompt(items))
}

fn prompt(items: List(Item)) -> String =
  items
    |> map(fn i: "- {i.title} ({i.source})")
    |> join("\n")
