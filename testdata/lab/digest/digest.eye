# The Ireland digest: every morning at 07:40 Irish time, read the day's news,
# have a model write a briefing, and send it through Notify. It is written once
# per Irish day: `/digest` sends today's again, and `/digest refresh` writes a
# new one.

import ai
import home.{UserId, dublin}
import miniflux
import notify
import telegram
import web

## A day's briefing, kept once it is written, so that asking for it again
## costs no model call.
entity Day(date: Date) {
  state article: Option(Article) = None
  ## The keys it was sent with: the schedule's, and one per /digest.
  state sent: Set(Key) = {}

  pub fn keep(a: Article) { article = Some(a) }

  pub fn sentAs(key: Key) { sent = sent |> add(key) }
}

type Article = { day: Date, title: String, document: String }

type DigestError =
  ## Miniflux stayed paused for six hours.
  | MinifluxAway
  | WriteFailed(ai.AiError)
  | NotSent(error: String)
  ## Notify queued it for nobody: subscribe someone to digest.* in Notify.
  | NoReaders
  ## Notify is still admitting it; sending again with the same key settles it.
  | StillPending(action: String)
  ## The writing run ended without a result: a deadline, a budget, a crash.
  | Faulted(Fault)

# ---------------------------------------------------------------- the schedule

## The morning's briefing. Quiet hours hold it until 08:00, since it goes at
## normal priority.
@on(daily(07:40, in: dublin))
@deadline(8 h)
job morning(day: Date) -> Result(notify.Delivery, DigestError) {
  match written!(day, refresh: false) {
    Ok(article) => deliver!(article, priority: Normal, key: "ireland-{day}")
    Err(e) => {
      # Say so where the briefing would have arrived.
      notify.send!(
        event: "digest.ireland",
        title: "Ireland digest not sent",
        body: "This morning's digest failed: {e}. Send /digest refresh to try again.",
        key: "ireland-{day}-failed",
      )
      Err(e)
    }
  }
}

# ---------------------------------------------------------------- /digest

## `/digest` sends today's briefing again, and `/digest refresh` writes a new
## one. Only a private chat with someone who receives the digest gets an
## answer; anyone else gets silence.
@on(event(telegram.updates))
job command(update: telegram.Update) {
  let Command(name: "digest", args) = update.body else { return }
  if !update.private || update.from != Some(update.chat) { return }
  if now!() - update.sentAt > 10 min { return }  # answered late is worse than not at all
  let Some(person) = notify.identity!(update.chat, event: "digest.ireland") else { return }
  let refresh = args |> trim |> lower == "refresh"
  let day = update.sentAt.date(in: dublin)
  let key = "digest-{update.id}"
  match written!(day, refresh:) {
    Ok(article) => deliver!(article, priority: High, to: Some([person]), key:)
    Err(e) => notify.send!(
      event: "digest.ireland",
      title: "Ireland digest",
      body: "The digest couldn't be written: {e}. Try again in a few minutes.",
      priority: High,
      to: Some([person]),
      key: "{key}-failed",
    )
  }
}

# ---------------------------------------------------------------- the steps

## Today's briefing: the one already written, or a new one.
fn written(day: Date, refresh: Bool) -> Result(Article, DigestError) =
  match Day(day).read!().article {
    Some(article) if !refresh => Ok(article)
    _ => match write.start(day).join!() {
      Completed(result) => result
      Failed(fault) => Err(Faulted(fault))
    }
  }

## The expensive part: read the sources and call the model. A run of its own,
## so that a fault in it (a deadline, the budget) still lets the caller report.
@memory(256 MiB)
@budget(usd: 0.50)
@deadline(7 h)
job write(day: Date) -> Result(Article, DigestError) {
  # Miniflux pauses while the game server or a CI check needs the memory.
  # Wait for it rather than lose the day, then let it catch up.
  if miniflux.me!() |> isErr {
    if !poll!(every: 1 min, within: 6 h, fn { miniflux.me!() |> isOk }) {
      return Err(MinifluxAway)
    }
    miniflux.refresh!()
    sleep!(5 min)
  }
  let news = gather!(since: now!() - 24 h)
  let briefing: Briefing = ai.generate!(
    model: env.get("DIGEST_MODEL") ?? "gpt-5.6-terra",
    system: prompt,
    prompt: news.sections |> toJson,
  ).mapErr(WriteFailed)?
  let article: Article = {
    day,
    title: "Ireland & Dublin — {day.format("%A %-d %B")}",
    document: render(briefing, news),
  }
  Day(day).keep!(article)
  Ok(article)
}

fn deliver(
  article: Article,
  priority: notify.Priority,
  to: Option(List(UserId)) = None,
  key: Key,
) -> Result(notify.Delivery, DigestError) {
  let delivery = notify.send!(
    event: "digest.ireland",
    title: article.title,
    body: article.document,
    priority:,
    format: Markdown,
    to:,
    key:,
  ).reconcile!()
  match delivery {
    Accepted(deliveries: 0, deduped: false, ..) => Err(NoReaders)
    Accepted(..) => {
      Day(article.day).sentAs!(key)
      Ok(delivery)
    }
    Pending(action) => Err(StillPending(action))
    Refused(error) => Err(NotSent(error))
  }
}

# ---------------------------------------------------------------- the news

## A section of the briefing and where its news comes from.
type Section = {
  heading: String,
  ## What belongs in the section, and how many stories.
  brief: String,
  category: miniflux.CategoryId,
  pages: List(Page),
}

type Page = { name: String, url: String, via: web.Route }

## What the model is given for one section.
type SectionNews = {
  heading: String,
  brief: String,
  items: List(Item),
  pages: List(PageText),
}

## A feed entry, as the model is given it.
type Item = { source: String, title: String, url: String, published: Instant, text: String }

type PageText = { source: String, url: String, text: String }

type News = {
  sections: List(SectionNews),
  ## A page that couldn't be read is reported, and never stops the digest.
  problems: List(String),
}

let sections: List(Section) = [
  {
    heading: "Temporary protection",
    brief: "Changes to the rules, payments and accommodation for people from Ukraine. Every story that matters.",
    category: miniflux.CategoryId(1),
    pages: [{ name: "Department of Justice", url: "https://www.irishimmigration.ie/", via: Vpn }],
  },
  {
    heading: "Dublin",
    brief: "What changes life in the city: transport, housing, services. At most five stories.",
    category: miniflux.CategoryId(4),
    pages: [],
  },
  {
    heading: "Coming up in Dublin",
    brief: "Events from today through the next seven days. At most five.",
    category: miniflux.CategoryId(5),
    pages: [{ name: "Dublin City Council events", url: "https://www.dublincity.ie/events", via: VpnBrowser }],
  },
]

fn gather(since: Instant) -> News {
  var found: List(SectionNews) = []
  var problems: List(String) = []
  for section in sections {
    var pages: List(PageText) = []
    for page in section.pages {
      match web.page!(page.url, via: page.via) {
        Ok(text) => pages += [{ source: page.name, url: page.url, text: text |> clip(12_000) }]
        Err(e) => problems += ["{page.name}: {e}"]
      }
    }
    found += [{
      heading: section.heading,
      brief: section.brief,
      items: miniflux.entries!(section.category, publishedAfter: since.unix(), limit: 60).entries
      |> map(item),
      pages,
    }]
  }
  { sections: found, problems }
}

fn item(e: miniflux.Entry) -> Item = {
  source: e.feed.title,
  title: e.title,
  url: e.url,
  published: e.publishedAt,
  text: e.content |> clip(300),
}

fn clip(text: String, limit: Int) -> String =
  if text |> size <= limit then text else "{text |> take(limit)}…"

# ---------------------------------------------------------------- the briefing

## A morning briefing for one reader.
type Briefing = {
  ## At most three: the most important things today, across all sections.
  highlights: List(String),
  ## One per section given, with exactly its heading, in the given order.
  sections: List(BriefingSection),
}

type BriefingSection = {
  heading: String,
  ## Empty when nothing in the section matters today.
  stories: List(Story),
}

type Story = {
  ## A short headline.
  headline: String,
  ## One to three plain sentences: what happened, and what it means for the reader.
  text: String,
  ## Set when the reader may need to do something: what, and by when.
  action: Option(String),
  ## Every source that reported the story.
  sources: List(Link),
}

type Link = { title: String, url: String }

let prompt = """
  You write a morning briefing for one reader, who moved to Dublin recently and
  lives in Ireland under EU temporary protection as a person from Ukraine. They
  want to know what changed that could affect their legal status, money, home
  and city, without reading the news all day.

  The input lists the briefing's sections in order. Each has a heading, a brief
  saying what belongs in it, feed items and the text of pages without a feed.

  - Use only facts in the input. Never guess dates, numbers or rules.
  - From pages, take only items dated within the last day or, for events, from
    today through the next seven days.
  - Merge reports of the same story into one, with every source.
  - Follow each section's brief on how many stories to give. Don't pad.
  - Leave out sport results, celebrity news, crime without wider consequence,
    opinion columns and advertising.
  - Plain, calm English. The whole briefing reads in five minutes.
  """

fn render(b: Briefing, news: News) -> String {
  var out = b.highlights |> map(fn h: "- {h}") |> join("\n")
  for section in b.sections {
    out += "\n\n## {section.heading}\n"
    if section.stories == [] {
      out += "\n- Nothing new."
    }
    for story in section.stories {
      let action = match story.action {
        Some(what) => "**Action:** {what} "
        None => ""
      }
      let sources = story.sources |> map(fn l: "[{l.title}]({l.url})") |> join(", ")
      out += "\n- {action}**{story.headline}** {story.text} {sources}"
    }
  }
  if news.problems != [] {
    out += "\n\n## Sources not read\n\n{news.problems |> map(fn p: "- {p}") |> join("\n")}"
  }
  out
}

# ---------------------------------------------------------------- tests

let today: Article = {
  day: 2026-10-12,
  title: "Ireland & Dublin — Monday 12 October",
  document: "- Rent cap extended",
}

fn digestCommand(chat: Int, from: Int) -> telegram.Update = {
  id: telegram.UpdateId(7),
  chat: telegram.ChatId(chat),
  private: chat == from,
  from: Some(telegram.ChatId(from)),
  sentAt: now(),
  body: Command(name: "digest", args: ""),
}

test "/digest sends today's briefing again without writing it" {
  with fakes(notify, from: "recordings/")
  with clock(2026-10-12 09:00 in dublin)
  Day(2026-10-12).keep(today)
  telegram.updates.publish(digestCommand(42, from: 42))
  expect(Day(2026-10-12).sent == toSet(["digest-7"]))
}

test "the morning's briefing goes at 07:40 Irish time, for the Irish day" {
  with fakes(notify, from: "recordings/")
  with clock(2026-10-12 06:30 in "UTC")  # 07:30 in Dublin, on summer time
  Day(2026-10-12).keep(today)
  advance(9 min)
  expect(Day(2026-10-12).sent == {})
  advance(1 min)
  expect(Day(2026-10-12).sent == toSet(["ireland-2026-10-12"]))
}

test "/digest in a group chat gets silence" {
  with fakes(notify, from: "recordings/")
  with clock(2026-10-12 09:00 in dublin)
  Day(2026-10-12).keep(today)
  telegram.updates.publish(digestCommand(-100, from: 42))
  expect(Day(2026-10-12).sent == {})
}

test "a page that can't be read is reported, not fatal" {
  let news: News = {
    sections: [],
    problems: ["Dublin City Council events: Blocked"],
  }
  let text = render({ highlights: ["Rent cap extended"], sections: [] }, news)
  expect(text |> contains("## Sources not read"))
  expect(text |> contains("- Dublin City Council events: Blocked"))
}
