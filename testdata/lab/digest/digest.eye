# The Ireland digest: every morning at 07:40 Irish time, read the day's news,
# have a model write a briefing, and send it through Notify. It is written once
# per Irish day: `/digest` sends today's again, and `/digest refresh` writes a
# new one.

import ai
import flaresolverr  # the tests fake it and http, which web reads pages with
import home.{UserId, dublin}
import http
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
  let dest = channel!()
  match written!(day, refresh: false) {
    Ok(article) => deliver!(article, dest, priority: Normal, only: dest.people, key: "{dest.keys}ireland-{day}")
    Err(e) => {
      # Say so where the briefing would have arrived.
      notify.send!(
        event: dest.event,
        title: "Ireland digest not sent",
        body: "This morning's digest failed: {e}. Send /digest refresh to try again.",
        to: dest.people,
        key: "{dest.keys}ireland-{day}-failed",
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
  let dest = channel!()
  let Some(person) = notify.identity!(update.chat, event: dest.event) else { return }
  let refresh = args |> trim |> lower == "refresh"
  let day = update.sentAt.date(in: dublin)
  let key = "{dest.keys}digest-{update.id}"
  match written!(day, refresh:) {
    Ok(article) => deliver!(article, dest, priority: High, only: Some([person]), key:)
    Err(e) => notify.send!(
      event: dest.event,
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
    model: env.get!("DIGEST_MODEL") ?? "gpt-5.6-terra",
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

## Where the digest goes: the Notify event, the people it is narrowed to
## (None: everyone who receives the event), and what its keys start with.
type Channel = { event: String, people: Option(List(UserId)), keys: String }

## The digest's channel: the event digest.ireland, or the one DIGEST_EVENT
## names, for everyone who receives it, or only the Authentik uids that
## DIGEST_TO lists, separated by commas. A shadow run beside another digest
## sets them, such as DIGEST_EVENT=digest.ireland.eye and DIGEST_TO=<the
## operator's uid>. Its keys then start with the event, since Notify drops a
## key it has already seen from the same caller, whoever it was for.
fn channel() -> Channel {
  let event = match env.get!("DIGEST_EVENT") {
    Some(name) if name |> trim != "" => name |> trim
    _ => "digest.ireland"
  }
  let uids = (env.get!("DIGEST_TO") ?? "") |> split(",") |> map(fn u: u |> trim) |> filter(fn u: u != "")
  if uids |> size > 20 {
    unreachable("DIGEST_TO lists {uids |> size} people, and Notify sends to at most 20")
  }
  let people = if uids == [] then None else Some(uids |> map(fn u: UserId(u)))
  let shadow = event != "digest.ireland" || uids != []
  { event, people, keys: if shadow then "{event}:" else "" }
}

fn deliver(
  article: Article,
  dest: Channel,
  priority: notify.Priority,
  only: Option(List(UserId)) = None,
  key: Key,
) -> Result(notify.Delivery, DigestError) {
  let delivery = notify.send!(
    event: dest.event,
    title: article.title,
    body: article.document,
    priority:,
    format: Markdown,
    to: only,
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
        Ok(html) => {
          let text = plain(html, base: Some(page.url))
          if text |> size < 200 {
            problems += ["{page.name}: the page has almost no text; it may need a browser"]
          } else {
            pages += [{ source: page.name, url: page.url, text: text |> clip(12_000) }]
          }
        }
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
  text: e.content |> plain |> clip(300),
}

fn clip(text: String, limit: Int) -> String =
  if text |> size <= limit then text else "{text |> take(limit)}…"

# ---------------------------------------------------------------- HTML to text

# A port of plain() from the TypeScript digest (html.ts), itself a port of the
# Python runner's html.parser subclass. Offsets are bytes (SPECS.md §5); the
# names of tags and attributes are ASCII.

## Elements whose content is left out. Not `form`: ASP.NET sites such as
## enterprise.gov.ie put the whole page in one.
let skipped = toSet(["script", "style", "noscript", "svg", "template", "head", "nav", "footer", "iframe"])

## Elements that start a new line.
let blocks = toSet(
  "address article aside blockquote br dd div dl dt figcaption h1 h2 h3 h4 h5 h6 header hr li main ol p pre section table td th time tr ul"
  |> words,
)

## Elements whose content is raw text: a `<` inside them doesn't open a tag.
let raw = toSet(["script", "style"])

## Entities by name. Any other is left as it is written.
let entities: Map(String, String) = {
  "amp": "&",
  "lt": "<",
  "gt": ">",
  "quot": "\"",
  "apos": "'",
  "nbsp": " ",
  "ndash": "–",
  "mdash": "—",
  "hellip": "…",
  "lsquo": "‘",
  "rsquo": "’",
  "ldquo": "“",
  "rdquo": "”",
  "euro": "€",
  "pound": "£",
  "copy": "©",
  "reg": "®",
  "middot": "·",
  "bull": "•",
}

## Readable text from HTML, a line for each block, with spaces collapsed. With
## a base URL, a link becomes `[text](url)`, its target resolved against the
## base; links within the page, to scripts, mail or phones stay text.
fn plain(html: String, base: Option(String) = None) -> String {
  var parts: List(String) = []
  var link: Option(Link) = None  # the link being read: its text so far, and its target
  var skip = 0                   # how many skipped elements we are inside
  var read = 0                   # the text before this has been read
  var look = 0                   # where the next tag may start
  while true {
    let Some(open) = html |> indexOf("<", from: look) else { break }
    let Some(tag) = tagAt(html, open) else {
      look = open + 1
      continue
    }
    if skip == 0 { keep(html.slice(read..open) ?? "", mut parts, mut link) }
    read = tag.end
    look = tag.end
    if tag.name == "" { continue }
    if !tag.closing && raw |> contains(tag.name) {
      # Go to its end tag without reading what is inside; the end tag leaves it.
      skip += 1
      read = closeOf(html, tag.name, from: tag.end) ?? html.byteSize()
      look = read
    } else if skipped |> contains(tag.name) {
      skip = if tag.closing then max(0, skip - 1) else skip + 1
    } else if blocks |> contains(tag.name) {
      parts += ["\n"]
    } else if tag.name == "a" && !tag.closing && skip == 0 && base != None {
      let target = hrefOf(tag.attributes)
      let lowered = target |> lower
      let local = ["#", "javascript:", "mailto:", "tel:"] |> any(fn p: lowered |> startsWith(p))
      if target != "" && !local {
        link = match resolve(target, against: base ?? "") {
          Some(url) => Some({ title: "", url })
          None => None
        }
      }
    } else if tag.name == "a" && tag.closing && link != None {
      let Some(l) = link else { continue }
      let text = l.title |> words |> join(" ")
      parts += [if text == "" then " " else " [{text}]({l.url}) "]
      link = None
    }
  }
  if skip == 0 { keep(html.slice(read..html.byteSize()) ?? "", mut parts, mut link) }
  parts
    |> join("")
    |> split("\n")
    |> map(fn line: line |> words |> join(" "))
    |> filter(fn line: line != "")
    |> join("\n")
}

## Adds text read between tags to the link being read, or else to the parts.
fn keep(html: String, mut parts: List(String), mut link: Option(Link)) {
  if html == "" { return }
  let text = decode(html)
  match link {
    Some(l) => link = Some({ l with title: l.title + text })
    None => parts += [text]
  }
}

## A tag, a comment or a declaration (`<!DOCTYPE html>`), which has no name.
type Tag = { name: String, closing: Bool, attributes: String, end: Int }

## The tag that starts at `<` at offset at, if one does.
fn tagAt(html: String, at: Int) -> Option(Tag) {
  if html.slice(at..at + 4) == Some("<!--") {
    match html |> indexOf("-->", from: at + 4) {
      Some(close) => return Some({ name: "", closing: false, attributes: "", end: close + 3 })
      None => {}  # unclosed: read as a declaration, up to the next >
    }
  }
  if byteAt(html, at + 1) == "!" {
    let close = html |> indexOf(">", from: at + 2)?
    return Some({ name: "", closing: false, attributes: "", end: close + 1 })
  }
  let closing = byteAt(html, at + 1) == "/"
  let start = if closing then at + 2 else at + 1
  if !isLetter(byteAt(html, start)) { return None }
  var i = start + 1
  while isNameByte(byteAt(html, i)) { i += 1 }
  let close = html |> indexOf(">", from: i)?
  Some({
    name: (html.slice(start..i) ?? "") |> lower,
    closing,
    attributes: html.slice(i..close) ?? "",
    end: close + 1,
  })
}

## Where the end tag of a raw element starts, in any case: `</script`.
fn closeOf(html: String, name: String, from: Int) -> Option(Int) {
  var look = from
  while true {
    let at = html |> indexOf("</", from: look)?
    let found = html.slice(at + 2..at + 2 + name.byteSize()) ?? ""
    if found |> lower == name { return Some(at) }
    look = at + 2
  }
}

## The value of a tag's href attribute, decoded and trimmed, or "".
fn hrefOf(attributes: String) -> String {
  # Offsets in the lower-case copy are the same as in the original, except
  # after the few characters whose lower case is longer or shorter.
  let lowered = attributes |> lower
  var look = 0
  while true {
    let Some(at) = lowered |> indexOf("href", from: look) else { return "" }
    look = at + 4
    if at > 0 && isWordByte(byteAt(attributes, at - 1)) { continue }
    var i = skipSpace(attributes, at + 4)
    if byteAt(attributes, i) != "=" { continue }
    i = skipSpace(attributes, i + 1)
    let quote = byteAt(attributes, i)
    let value = if quote == "\"" || quote == "'" {
      let close = attributes |> indexOf(quote, from: i + 1) ?? attributes.byteSize()
      attributes.slice(i + 1..close) ?? ""
    } else {
      var end = i
      while end < attributes.byteSize() && !isSpace(byteAt(attributes, end)) { end += 1 }
      attributes.slice(i..end) ?? ""
    }
    return value |> decode |> trim
  }
}

## Text with its character references replaced: `&amp;`, `&#8364;`, `&#x2014;`.
## A reference that names nothing stays as it is written.
fn decode(text: String) -> String {
  if !(text |> contains("&")) { return text }
  var parts: List(String) = []
  var read = 0
  while true {
    let Some(amp) = text |> indexOf("&", from: read) else { break }
    parts += [text.slice(read..amp) ?? ""]
    let (found, end) = entityAt(text, amp + 1)
    parts += [found ?? (text.slice(amp..end) ?? "")]
    read = end
  }
  parts += [text.slice(read..text.byteSize()) ?? ""]
  parts |> join("")
}

## What the reference after a `&` at offset at stands for, if anything, and
## where it ends, after its optional `;`.
fn entityAt(text: String, at: Int) -> (Option(String), Int) {
  let numeric = byteAt(text, at) == "#"
  let hex = numeric && (byteAt(text, at + 1) |> lower) == "x"
  let start = if hex then at + 2 else if numeric then at + 1 else at
  var i = start
  while referenceByte(byteAt(text, i), hex:, numeric:) { i += 1 }
  if i == start { return (None, at) }
  let name = text.slice(start..i) ?? ""
  let end = if byteAt(text, i) == ";" then i + 1 else i
  let value = if numeric {
    match parseInt(name, base: if hex then 16 else 10) {
      Ok(code) if code > 0 => fromCode(code) |> map(fn c: "{c}")
      _ => None
    }
  } else {
    entities |> get(name |> lower)
  }
  (value, end)
}

## The resolved URL of a link's target, given the URL of the page it is on.
fn resolve(target: String, against: String) -> Option(String) {
  if hasScheme(target) { return Some(target) }
  let page = before(before(against, "#"), "?")
  let schemeEnd = page |> indexOf("://")?
  let pathStart = page |> indexOf("/", from: schemeEnd + 3) ?? page.byteSize()
  let origin = page.slice(0..pathStart)?
  let path = page.slice(pathStart..page.byteSize())?
  if target |> startsWith("//") { return Some("{page.slice(0..schemeEnd + 1)?}{target}") }
  if target |> startsWith("/") { return Some(origin + target) }
  if target |> startsWith("?") { return Some(origin + path + target) }
  # The directory: the path up to its last slash.
  var dirs = path |> split("/") |> reverse |> drop(1) |> reverse
  var rest = target
  while true {
    if rest |> startsWith("./") {
      rest = rest |> drop(2)
    } else if rest |> startsWith("../") {
      rest = rest |> drop(3)
      if dirs |> size > 1 { dirs = dirs |> reverse |> drop(1) |> reverse }
    } else {
      break
    }
  }
  Some("{origin}{dirs |> join("/")}/{rest}")
}

## Whether a link's target starts with a scheme, such as `https:`.
fn hasScheme(target: String) -> Bool {
  let Some(colon) = target |> indexOf(":") else { return false }
  let name = target.slice(0..colon) ?? ""
  colon > 0 && isLetter(byteAt(target, 0)) && name |> chars |> all(fn c: isSchemeChar("{c}"))
}

## s up to the first mark in it, or all of s.
fn before(s: String, mark: String) -> String =
  s.slice(0..(s |> indexOf(mark) ?? s.byteSize())) ?? s

## The one-byte text at offset i, or "" past the end or inside a character.
fn byteAt(s: String, i: Int) -> String = if i < 0 then "" else s.slice(i..i + 1) ?? ""

fn referenceByte(c: String, hex: Bool, numeric: Bool) -> Bool =
  if hex then isHexDigit(c) else if numeric then isDigit(c) else isLetter(c)

fn isLetter(c: String) -> Bool = (c >= "a" && c <= "z") || (c >= "A" && c <= "Z")
fn isDigit(c: String) -> Bool = c >= "0" && c <= "9"
fn isHexDigit(c: String) -> Bool = isDigit(c) || (c >= "a" && c <= "f") || (c >= "A" && c <= "F")
fn isWordByte(c: String) -> Bool = isLetter(c) || isDigit(c) || c == "_"
fn isNameByte(c: String) -> Bool = isWordByte(c) || c == "-"
fn isSchemeChar(c: String) -> Bool = isWordByte(c) || c == "+" || c == "-" || c == "."
fn isSpace(c: String) -> Bool = c == " " || c == "\t" || c == "\n" || c == "\r" || c == "\u{C}"

fn skipSpace(s: String, from: Int) -> Int {
  var i = from
  while isSpace(byteAt(s, i)) { i += 1 }
  i
}

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

test "the morning reads its sources, has the briefing written, and sends it" {
  with fakes(notify, miniflux, flaresolverr, http, ai, from: "recordings/")
  with clock(2026-10-12 06:30 in "UTC")  # 07:30 in Dublin
  advance(10 min)
  expect(Day(2026-10-12).sent == toSet(["ireland-2026-10-12"]))
  let Some(article) = Day(2026-10-12).article else { unreachable("the morning wrote no briefing") }
  expect(article.title == "Ireland & Dublin — Monday 12 October")
  expect(article.document |> contains("- Temporary protection is extended to 4 March 2027"))
  expect(article.document |> contains("**Rent cap extended** Rent pressure zones now run to 2027. [RTÉ](https://www.rte.ie/news/rent)"))
  # The council's events page needs a browser that didn't get past it.
  expect(article.document |> contains("- Dublin City Council events: Unsolved(message: \"Challenge not detected\")"))
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

test "entities decode, and unknown ones stay" {
  expect(decode("Fish &amp; chips &#8364;5 &#x2014; &euro; &bogus;") == "Fish & chips €5 — € &bogus;")
  expect(decode("AT&T &#0; &") == "AT&T &#0; &")
}

test "blocks become lines, and skipped elements vanish" {
  let html = """
    <head><title>T</title></head><nav>Menu</nav><h1>Head</h1><p>One <b>two</b></p>
    <script>if (a < b) \{ x = "</p>"; \}</script><footer>Foot</footer><div>Three</div>
    """
  expect(plain(html) == "Head\nOne two\nThree")
  expect(plain("<nav><SCRIPT>x</SCRIPT>Menu</nav><p>After</p>") == "After")
  expect(plain("<!-- <p>gone</p> --><!DOCTYPE html>a < b") == "a < b")
}

test "links keep their target only with a base, and never for in-page or script links" {
  let html = "<p><a href=\"/a?x=1&amp;y=2\">First  link</a> <a href=\"#top\">Top</a> <a HREF='javascript:x()'>J</a></p>"
  expect(plain(html, base: Some("https://site.example/news/")) == "[First link](https://site.example/a?x=1&y=2) Top J")
  expect(plain(html) == "First link Top J")
  expect(plain("<a href=more>More</a>", base: Some("https://site.example/news/today?page=2")) == "[More](https://site.example/news/more)")
  expect(plain("<a xhref=x href=../up>Up</a>", base: Some("https://site.example/a/b/")) == "[Up](https://site.example/a/up)")
}

test "a form still yields its text, since ASP.NET pages live in one" {
  expect(plain("<form><p>Notice</p></form>") == "Notice")
}
