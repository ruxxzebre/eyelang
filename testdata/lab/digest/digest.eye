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

  pub fn keep(a: Article) { article = Some(a) }
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
  ## The model wrote no story and no highlight, though it was given `items`
  ## feed items: a failed call, not a quiet day, so nothing is sent.
  | EmptyBriefing(items: Int)

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
      # Say so where the briefing would have arrived. The run fails with e
      # either way, so a refusal of the notice changes nothing.
      let _ = notify.send!(
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
  # An answer that can't be delivered has nobody else to tell.
  match written!(day, refresh:) {
    Ok(article) => {
      let _ = deliver!(article, dest, priority: High, only: Some([person]), key:)
    }
    Err(e) => {
      let _ = notify.send!(
        event: dest.event,
        title: "Ireland digest",
        body: "The digest couldn't be written: {e}. Try again in a few minutes.",
        priority: High,
        to: Some([person]),
        key: "{key}-failed",
      )
    }
  }
}

# ---------------------------------------------------------------- the steps

## Today's briefing: the one already written, or a new one.
fn written(day: Date, refresh: Bool) -> Result(Article, DigestError) =
  match Day(day).read!().article {
    Some(article) if !refresh => Ok(article)
    _ => match write.start!(day).join!() {
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
    # A refresh that didn't happen only leaves older news to read.
    let _ = miniflux.refresh!()
    sleep!(5 min)
  }
  let until = now!()
  let news = gather!(since: until - 24 h, until:)
  let briefing: Briefing = ai.generate!(
    model: env.get!("DIGEST_MODEL") ?? "gpt-5.6-terra",
    system: prompt,
    prompt: news.input |> toJson,
    effort: Some(Medium),  # as the TypeScript digest asks
  ).mapErr(WriteFailed)?
  let items = news.input.sections |> map(fn s: s.items |> size) |> sum
  if items > 0 && briefing.highlights == [] && !(briefing.sections |> any(fn s: s.stories != [])) {
    return Err(EmptyBriefing(items:))
  }
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
    Accepted(..) => Ok(delivery)
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
  ## The title of its Miniflux category, made if it is missing. A feed added
  ## to that category in Miniflux joins the section.
  category: String,
  feeds: List(Source),
  pages: List(Page),
}

## A feed the digest subscribes the section's category to, if Miniflux
## doesn't have it. It never removes or moves one, so a feed deleted in
## Miniflux stays deleted until it is listed again.
type Source = { name: String, url: String }

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

## What the model is given: today's date, the window items were published
## in, and the sections in order.
type Input = { today: String, window: String, sections: List(SectionNews) }

type News = {
  input: Input,
  ## A page or feed that couldn't be read is reported, and never stops the
  ## digest.
  problems: List(String),
}

# The sections, their categories, feeds and pages are those of the TypeScript
# digest's sources.json, in its order.
let sections: List(Section) = [
  {
    heading: "Temporary protection (Ukrainians)",
    brief: "Temporary protection for people from Ukraine in Ireland and the EU: permission renewals and expiry, IRP cards, the Temporary Protection Transition Scheme (a Stamp 4 permission) and when it opens, the end of State-contracted accommodation, the Accommodation Recognition Payment, social welfare and payments, travel, and statements by the Irish government, the EU or Ukrainian community organisations. One page lists Immigration Service pages with the time each was last modified: mention a page on temporary protection or Ukraine modified within the window, since that is how rule changes appear. The Ukraine updates page matters only if it says it was updated within the window. Council of the EU and European Commission items on other subjects do not belong here.",
    category: "Digest: Temporary protection",
    feeds: [
      { name: "Immigration Service Delivery", url: "https://www.irishimmigration.ie/feed/" },
      { name: "European Commission Home Affairs", url: "https://home-affairs.ec.europa.eu/node/2/rss_en" },
      { name: "Council of the EU", url: "https://www.consilium.europa.eu/en/rss/pressreleases.ashx" },
      {
        name: "Google News: temporary protection in Ireland",
        url: "https://news.google.com/rss/search?q=%22temporary+protection%22+(Ukrainian+OR+Ukrainians)+Ireland+when:7d&hl=en-IE&gl=IE&ceid=IE:en",
      },
      { name: "TheJournal: Ukraine", url: "https://www.thejournal.ie/topic/ukraine/feed/" },
    ],
    pages: [
      {
        name: "Immigration Service pages changed",
        url: "https://www.irishimmigration.ie/wp-json/wp/v2/pages?orderby=modified&order=desc&per_page=15&_fields=modified,link,title",
        via: Direct,
      },
      {
        name: "gov.ie: Justice on Ukraine",
        url: "https://www.gov.ie/en/search/?q=Ukraine&organisation=Department+of+Justice%2C+Home+Affairs+and+Migration",
        via: Direct,
      },
      {
        name: "gov.ie: Ukraine updates",
        url: "https://www.gov.ie/en/department-of-justice-home-affairs-and-migration/campaigns/ukraine-updates-en/",
        via: Direct,
      },
      { name: "Irish Refugee Council", url: "https://www.irishrefugeecouncil.ie/media-centre/press-releases/", via: Direct },
      { name: "Embassy of Ukraine in Ireland", url: "https://ireland.mfa.gov.ua/en/news", via: VpnBrowser },
    ],
  },
  {
    heading: "Immigration",
    brief: "Immigration law and practice in Ireland beyond temporary protection: new laws and bills, visas, residence permissions and stamps, registration and renewals, citizenship and naturalisation (including any change to the years of residence required), employment permits and the critical skills list, processing times and office changes.",
    category: "Digest: Immigration",
    feeds: [
      { name: "Migrant Rights Centre Ireland", url: "https://www.mrci.ie/feed/" },
      { name: "Immigrant Council of Ireland", url: "https://www.immigrantcouncil.ie/rss.xml" },
      { name: "TheJournal: immigration", url: "https://www.thejournal.ie/topic/immigration/feed/" },
      {
        name: "Google News: Irish immigration",
        url: "https://news.google.com/rss/search?q=Ireland+(immigration+OR+%22employment+permit%22+OR+citizenship+OR+%22international+protection%22)+when:2d&hl=en-IE&gl=IE&ceid=IE:en",
      },
    ],
    pages: [
      {
        name: "gov.ie: Justice press releases",
        url: "https://www.gov.ie/en/search/?category=Press+Release&organisation=Department+of+Justice%2C+Home+Affairs+and+Migration",
        via: Direct,
      },
      {
        name: "Employment permits: latest updates",
        url: "https://enterprise.gov.ie/en/what-we-do/workplace-and-skills/employment-permits/latest-updates/",
        via: Direct,
      },
    ],
  },
  {
    heading: "Ireland",
    brief: "National news that matters to someone living in Ireland: government decisions, the Budget, taxes and prices, health services, housing policy, strikes, public safety and major national events. The reader wants to keep up with the country, not only with what affects them: give the five to eight most significant stories of the day; even a quiet day has some.",
    category: "Digest: Ireland",
    feeds: [
      { name: "RTÉ News", url: "https://www.rte.ie/feeds/rss/?index=/news/" },
      { name: "TheJournal", url: "https://www.thejournal.ie/feed/" },
      { name: "The Irish Times", url: "https://www.irishtimes.com/arc/outboundfeeds/feed-irish-news/?from=0&size=50" },
      { name: "Irish Independent", url: "https://www.independent.ie/irish-news/rss/" },
      { name: "Business Post", url: "https://www.businesspost.ie/rss-feed/news/" },
    ],
    pages: [],
  },
  {
    heading: "Dublin",
    brief: "Dublin city and county: council decisions, transport projects, housing, safety, public services, closures and new developments. Give the three to six most significant stories.",
    category: "Digest: Dublin",
    feeds: [
      { name: "DublinLive", url: "https://www.dublinlive.ie/news/dublin-news/?service=rss" },
      { name: "TheJournal: Dublin", url: "https://www.thejournal.ie/topic/dublin/feed/" },
      { name: "RTÉ News: Dublin", url: "https://www.rte.ie/feeds/rss/?index=/news/dublin/" },
      { name: "Dublin Inquirer", url: "https://www.dublininquirer.com/latest/rss/" },
    ],
    pages: [{ name: "Dublin City Council news", url: "https://www.dublincity.ie/news", via: Vpn }],
  },
  {
    heading: "Coming up in Dublin",
    brief: "Large events in Dublin from today through the next seven days: stadium and arena concerts, big matches, festivals, marathons and parades, public holidays and what changes on them, and the road closures they bring. Give the day, time and place, and say when one is cancelled or sold out.",
    category: "Digest: Dublin events",
    feeds: [{ name: "Dublin Inquirer: things to do", url: "https://www.dublininquirer.com/tag/things-to-do/rss/" }],
    pages: [
      { name: "dublin.ie: what's on", url: "https://dublin.ie/whats-on/", via: Direct },
      { name: "3Arena", url: "https://3arena.ie/search/events/P0/", via: Direct },
      { name: "Aviva Stadium", url: "https://www.avivastadium.ie/whats-on", via: Direct },
      { name: "3Olympia Theatre", url: "https://www.3olympia.ie/whats-on", via: Direct },
    ],
  },
  {
    heading: "Money & housing",
    brief: "Tax and Revenue, social welfare, the Budget, cost of living and energy bills, renting rules and rents, Residential Tenancies Board decisions, and scams aimed at renters or newcomers.",
    category: "Digest: Money and housing",
    feeds: [
      { name: "Residential Tenancies Board", url: "https://rtb.ie/feed/?post_type=news" },
      { name: "TheJournal: Budget 2027", url: "https://www.thejournal.ie/topic/budget-2027/feed/" },
      { name: "TheJournal: housing", url: "https://www.thejournal.ie/topic/housing/feed/" },
      { name: "TheJournal: cost of living", url: "https://www.thejournal.ie/topic/cost-of-living/feed/" },
    ],
    pages: [
      { name: "Citizens Information: what's new", url: "https://www.citizensinformation.ie/en/whats-new/", via: Direct },
      {
        name: "gov.ie: Social Protection press releases",
        url: "https://www.gov.ie/en/search/?category=Press+Release&organisation=Department+of+Social+Protection",
        via: Direct,
      },
    ],
  },
  {
    heading: "Transport & weather",
    brief: "Disruption to Dublin Bus, Luas, DART, Irish Rail and roads: strikes, timetable changes and closures; and Met Éireann weather warnings that are orange or red anywhere, or any colour for Dublin. The Luas feed lists disruptions in force; the Met Éireann Dublin page lists warnings in force. Leave out marine warnings.",
    category: "Digest: Transport and weather",
    feeds: [
      { name: "Luas", url: "https://www.luas.ie/rss.xml" },
      { name: "Met Éireann warnings", url: "https://www.met.ie/warningsxml/rss.xml" },
      { name: "TheJournal: public transport", url: "https://www.thejournal.ie/topic/public-transport/feed/" },
    ],
    pages: [
      { name: "Met Éireann: Dublin warnings", url: "https://www.met.ie/Open_Data/json/warning_EI07.json", via: Direct },
      { name: "Irish Rail news", url: "https://www.irishrail.ie/en-ie/news", via: Direct },
      { name: "Transport for Ireland news", url: "https://www.transportforireland.ie/news/", via: Vpn },
      { name: "Dublin Bus news", url: "https://www.dublinbus.ie/news", via: VpnBrowser },
    ],
  },
]

# ---------------------------------------------------------------- Miniflux

## The sections' categories, by title, and what went wrong on the way.
type Categories = { ids: Map(String, miniflux.CategoryId), problems: List(String) }

## Each section's Miniflux category, found by its exact title and made if it
## is missing, with the section's feeds subscribed where Miniflux doesn't have
## them, matched by URL. Miniflux may keep the URL a redirect led to, so its
## refusing a feed that already exists isn't a problem. The problems also
## list the sections' feeds that Miniflux can't read.
fn categories() -> Categories {
  var ids: Map(String, miniflux.CategoryId) = {}
  for c in miniflux.categories!() {
    ids = ids |> insert(c.title, c.id)
  }
  let subscribed = miniflux.feeds!()
  var known = subscribed |> map(fn f: f.feedUrl) |> toSet
  var problems: List(String) = []
  for section in sections {
    let id = match ids |> get(section.category) {
      Some(id) => id
      None => made!(section.category)
    }
    ids = ids |> insert(section.category, id)
    for feed in section.feeds {
      if known |> contains(feed.url) { continue }
      match miniflux.subscribe!(feedUrl: feed.url, categoryId: id) {
        Added(_) => known = known |> add(feed.url)
        NotAdded(error) if error |> contains("already exists") => ()
        NotAdded(error) => problems += ["{feed.name}: Miniflux could not subscribe ({error})"]
        Uncertain(_) => problems += ["{feed.name}: Miniflux's answer to subscribing was lost"]
      }
    }
  }
  let ours = ids |> values |> toSet
  for f in subscribed {
    if ours |> contains(f.category.id) && f.parsingErrorCount > 0 {
      let why = if f.parsingErrorMessage == "" then "unreadable" else f.parsingErrorMessage
      problems += ["{f.title}: {why |> clip(120)}"]
    }
  }
  { ids, problems }
}

## A new category's id. When Miniflux's answer is lost, the categories say
## whether it arrived; only if it didn't is it made again.
fn made(title: String) -> miniflux.CategoryId =
  match miniflux.createCategory!(title:) {
    Uncertain(_) => match miniflux.categories!() |> find(fn c: c.title == title) {
      Some(c) => c.id
      None => match miniflux.createCategory!(title:) {
        Uncertain(_) => unreachable("Miniflux's answer to making the category {title} was lost twice")
        Created(c) => c.id
      }
    }
    Created(c) => c.id
  }

fn gather(since: Instant, until: Instant) -> News {
  let found = categories!()
  var news: List(SectionNews) = []
  var problems = found.problems
  for section in sections {
    let Some(id) = found.ids |> get(section.category) else {
      unreachable("no category for {section.heading}")
    }
    var seen: Set(String) = {}
    var items: List(Item) = []
    for e in miniflux.entries!(id, publishedAfter: since.unix(), limit: 60).entries {
      if seen |> contains(e.url) { continue }
      seen = seen |> add(e.url)
      items += [item(e)]
    }
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
    news += [{ heading: section.heading, brief: section.brief, items, pages }]
  }
  let input: Input = {
    today: until.date(in: dublin).format("%A %d %B %Y"),
    window: "{irishTime(since)} to {irishTime(until)} Irish time",
    sections: news,
  }
  { input, problems }
}

## An instant as the window gives it: `12 October 07:40`.
fn irishTime(at: Instant) -> String = "{at.date(in: dublin).format("%d %B")} {at.time(in: dublin).format("%H:%M")}"

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
  var read = html.start()        # the text before this has been read
  var look = html.start()        # where the next tag may start
  while true {
    let Some(open) = html |> indexOf("<", from: look) else { break }
    let Some(tag) = tagAt(html, open) else {
      look = past(html, open)
      continue
    }
    if skip == 0 { keep(html.slice(read..open) ?? "", mut parts, mut link) }
    read = tag.end
    look = tag.end
    if tag.name == "" { continue }
    if !tag.closing && raw |> contains(tag.name) {
      # Go to its end tag without reading what is inside; the end tag leaves it.
      skip += 1
      read = closeOf(html, tag.name, from: tag.end) ?? html.end()
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
  if skip == 0 { keep(rest(html, read), mut parts, mut link) }
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
type Tag = { name: String, closing: Bool, attributes: String, end: Offset }

## The tag that starts with the `<` at at, if one does.
fn tagAt(html: String, at: Offset) -> Option(Tag) {
  if rest(html, at) |> startsWith("<!--") {
    match html |> indexOf("-->", from: past(html, at, by: 4)) {
      Some(close) => return Some({ name: "", closing: false, attributes: "", end: past(html, close, by: 3) })
      None => {}  # unclosed: read as a declaration, up to the next >
    }
  }
  let second = past(html, at)
  if char(html, second) == '!' {
    let close = html |> indexOf(">", from: second)?
    return Some({ name: "", closing: false, attributes: "", end: past(html, close) })
  }
  let closing = char(html, second) == '/'
  let start = if closing then past(html, second) else second
  if !isLetter(char(html, start)) { return None }
  var i = past(html, start)
  while isNameChar(char(html, i)) { i = past(html, i) }
  let close = html |> indexOf(">", from: i)?
  Some({
    name: (html.slice(start..i) ?? "") |> lower,
    closing,
    attributes: html.slice(i..close) ?? "",
    end: past(html, close),
  })
}

## Where the end tag of a raw element starts, in any case: `</script`.
fn closeOf(html: String, name: String, from: Offset) -> Option(Offset) {
  var look = from
  while true {
    let at = html |> indexOf("</", from: look)?
    let start = past(html, at, by: 2)
    let found = html.slice(start..past(html, start, by: name |> size)) ?? ""
    if found |> lower == name { return Some(at) }
    look = start
  }
}

## The value of a tag's href attribute, decoded and trimmed, or "".
fn hrefOf(attributes: String) -> String {
  var at = attributes.start()
  var previous = '\0'
  while at < attributes.end() {
    let here = at
    let c = char(attributes, at)
    at = past(attributes, at)
    let starts = !isWordChar(previous) && (rest(attributes, here) |> take(4) |> lower) == "href"
    previous = c
    if !starts { continue }
    var i = skipSpace(attributes, past(attributes, here, by: 4))
    if char(attributes, i) != '=' { continue }
    i = skipSpace(attributes, past(attributes, i))
    let quote = char(attributes, i)
    let value = if quote == '"' || quote == '\'' {
      let open = past(attributes, i)
      let close = attributes |> indexOf("{quote}", from: open) ?? attributes.end()
      attributes.slice(open..close) ?? ""
    } else {
      var end = i
      while end < attributes.end() && !isSpace(char(attributes, end)) { end = past(attributes, end) }
      attributes.slice(i..end) ?? ""
    }
    return value |> decode |> trim
  }
  ""
}

## Text with its character references replaced: `&amp;`, `&#8364;`, `&#x2014;`.
## A reference that names nothing stays as it is written.
fn decode(text: String) -> String {
  if !(text |> contains("&")) { return text }
  var parts: List(String) = []
  var read = text.start()
  while true {
    let Some(amp) = text |> indexOf("&", from: read) else { break }
    parts += [text.slice(read..amp) ?? ""]
    let (found, end) = entityAt(text, past(text, amp))
    parts += [found ?? (text.slice(amp..end) ?? "")]
    read = end
  }
  parts += [rest(text, read)]
  parts |> join("")
}

## What the reference after a `&`, starting at at, stands for, if anything,
## and where it ends, after its optional `;`.
fn entityAt(text: String, at: Offset) -> (Option(String), Offset) {
  let numeric = char(text, at) == '#'
  let x = char(text, past(text, at))
  let hex = numeric && (x == 'x' || x == 'X')
  let start = if hex then past(text, at, by: 2) else if numeric then past(text, at) else at
  var i = start
  while referenceChar(char(text, i), hex:, numeric:) { i = past(text, i) }
  if i == start { return (None, at) }
  let name = text.slice(start..i) ?? ""
  let end = if char(text, i) == ';' then past(text, i) else i
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
  let page = upTo(upTo(against, "#"), "?")
  let schemeEnd = page |> indexOf("://")?
  let pathStart = page |> indexOf("/", from: past(page, schemeEnd, by: 3)) ?? page.end()
  let origin = page.slice(page.start()..pathStart)?
  let path = rest(page, pathStart)
  if target |> startsWith("//") { return Some("{page.slice(page.start()..schemeEnd)?}:{target}") }
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
  let name = target.slice(target.start()..colon) ?? ""
  isLetter(char(target, target.start())) && name |> chars |> all(fn c: isSchemeChar(c))
}

## s up to the first mark in it, or all of s.
fn upTo(s: String, mark: String) -> String =
  s.slice(s.start()..(s |> indexOf(mark) ?? s.end())) ?? s

## The rest of s from i.
fn rest(s: String, i: Offset) -> String = s.slice(i..s.end()) ?? ""

## The position by characters after i, or the end.
fn past(s: String, i: Offset, by: Int = 1) -> Offset = s |> after(i, by:) ?? s.end()

## The character at i, or `\0` at the end.
fn char(s: String, i: Offset) -> Char = s.charAt(i) ?? '\0'

fn referenceChar(c: Char, hex: Bool, numeric: Bool) -> Bool =
  if hex then isHexDigit(c) else if numeric then isDigit(c) else isLetter(c)

fn isLetter(c: Char) -> Bool = (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
fn isDigit(c: Char) -> Bool = c >= '0' && c <= '9'
fn isHexDigit(c: Char) -> Bool = isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
fn isWordChar(c: Char) -> Bool = isLetter(c) || isDigit(c) || c == '_'
fn isNameChar(c: Char) -> Bool = isWordChar(c) || c == '-'
fn isSchemeChar(c: Char) -> Bool = isWordChar(c) || c == '+' || c == '-' || c == '.'
fn isSpace(c: Char) -> Bool = c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\u{C}'

fn skipSpace(s: String, from: Offset) -> Offset {
  var i = from
  while isSpace(char(s, i)) { i = past(s, i) }
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

  The input is JSON: today's date, the window items were published in, and the
  briefing's sections in order. Each section has a heading, a brief saying what
  belongs in it, feed items (source, title, url, published time, opening text)
  and pages (a source page's text, links written [text](url)); a page lists
  items with their own dates.

  - Highlights: at most three, the most important things today across all
    sections.
  - One section for each section given, with exactly its heading, in the given
    order.
  - Each story has a short headline, then one to three plain sentences on what
    happened and what it means for the reader, and every source that reported
    it. Merge reports of the same story into one story with each source.
  - Set the action when the reader may need to do something (a deadline,
    renewal, application window, or a change to payments or accommodation),
    and say what and by when.
  - Use only facts in the input. Never guess dates, numbers or rules. When an
    item's opening text is too short to tell what it means, say only what it
    says.
  - From pages, take only items dated within the window, or, for events,
    happening from today through the next seven days.
  - Follow each section's brief on how many stories to give. A section with
    nothing that matters has no stories. Do not pad.
  - Leave out sport results, celebrity news, crime reports without wider
    consequence, opinion columns and advertising.
  - Plain, calm English; the whole briefing reads in five minutes.
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

## The keys Notify was sent briefings with.
fn keys(sent: Handler) -> List(Key) = sent.calls(notify.send) |> map(fn c: c.key)

test "/digest sends today's briefing again without writing it" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  with clock(2026-10-12 09:00 in dublin)
  Day(2026-10-12).keep(today)
  telegram.updates.publish(digestCommand(42, from: 42))
  expect(keys(sent) == ["digest-7"])
  let [briefing] = sent.calls(notify.send) else { unreachable("not sent once") }
  expect(briefing.title == today.title && briefing.priority == High && briefing.format == Markdown)
}

test "the morning's briefing goes at 07:40 Irish time, for the Irish day" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  with clock(2026-10-12 06:30 in "UTC")  # 07:30 in Dublin, on summer time
  Day(2026-10-12).keep(today)
  advance(9 min)
  expect(keys(sent) == [])
  advance(1 min)
  expect(keys(sent) == ["ireland-2026-10-12"])
}

test "the morning reads its sources, has the briefing written, and sends it" {
  let services = fakes(notify, miniflux, flaresolverr, http, ai, from: "recordings/")
  with services
  with clock(2026-10-12 06:30 in "UTC")  # 07:30 in Dublin
  advance(10 min)
  expect(keys(services) == ["ireland-2026-10-12"])
  let Some(article) = Day(2026-10-12).article else { unreachable("the morning wrote no briefing") }
  expect(article.title == "Ireland & Dublin — Monday 12 October")
  expect(article.document |> contains("- Temporary protection is extended to 4 March 2027"))
  expect(article.document |> contains("**Rent cap extended** Rent pressure zones now run to 2027. [RTÉ](https://www.rte.ie/news/rent)"))
  # The Embassy's page needs a browser that didn't get past it.
  expect(article.document |> contains("- Embassy of Ukraine in Ireland: the browser couldn't get past the page's checks: Challenge not detected"))
  # Miniflux can't read one of the sections' feeds; a feed in another
  # category isn't the digest's to report, and a feed refused as existing
  # isn't a problem.
  expect(article.document |> contains("- Luas: Get \"https://www.luas.ie/rss.xml\": 503 Service Unavailable"))
  expect(!(article.document |> contains("Not the digest's")))
  expect(!(article.document |> contains("could not subscribe")))
}

test "a section's category is made when Miniflux doesn't have it" {
  let services = fakes(notify, flaresolverr, http, ai, from: "recordings/")
  with services
  with fakes(miniflux, from: "recordings/created/")
  with clock(2026-10-12 06:30 in "UTC")
  advance(10 min)
  expect(keys(services) == ["ireland-2026-10-12"])
}

test "a feed Miniflux won't subscribe to is reported, and the digest goes on" {
  let services = fakes(notify, flaresolverr, http, ai, from: "recordings/")
  with services
  with fakes(miniflux, from: "recordings/refused/")
  with clock(2026-10-12 06:30 in "UTC")
  advance(10 min)
  expect(keys(services) == ["ireland-2026-10-12"])
  let Some(article) = Day(2026-10-12).article else { unreachable("the morning wrote no briefing") }
  expect(article.document |> contains("- Immigration Service Delivery: Miniflux could not subscribe (Unable to find a feed at this URL)"))
}

test "a reply that doesn't match the briefing's schema is an error, never an empty briefing" {
  let services = fakes(notify, miniflux, flaresolverr, http, from: "recordings/")
  with services
  with fakes(ai, from: "recordings/malformed/")
  with clock(2026-10-12 06:30 in "UTC")
  advance(10 min)
  expect(Day(2026-10-12).article == None)
  # Only the notice that this morning's digest failed goes out.
  expect(keys(services) == ["ireland-2026-10-12-failed"])
}

test "a briefing with nothing in it, from news with items, isn't sent" {
  let services = fakes(notify, miniflux, flaresolverr, http, from: "recordings/")
  with services
  with fakes(ai, from: "recordings/empty/")
  with clock(2026-10-12 06:30 in "UTC")
  advance(10 min)
  expect(Day(2026-10-12).article == None)
  # Only the notice that this morning's digest failed goes out.
  expect(keys(services) == ["ireland-2026-10-12-failed"])
}

test "/digest in a group chat gets silence" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  with clock(2026-10-12 09:00 in dublin)
  Day(2026-10-12).keep(today)
  telegram.updates.publish(digestCommand(-100, from: 42))
  expect(sent.calls(notify.send) == [] && sent.calls(notify.identity) == [])
}

test "a page that can't be read is reported, not fatal" {
  let news: News = {
    input: { today: "Monday 12 October 2026", window: "", sections: [] },
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
