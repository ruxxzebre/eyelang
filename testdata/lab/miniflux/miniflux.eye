# Miniflux, the feed reader the digest reads from. Lifecycle pauses it while
# the game server or a CI check needs its memory.

import http

pub type CategoryId = | CategoryId(Int)

pub type Feed = { title: String }

pub type Entry = {
  title: String,
  url: String,
  publishedAt: Instant,
  ## The entry's text, as HTML.
  content: String,
  feed: Feed,
}

pub type Entries = { total: Int, entries: List(Entry) }

pub type User = { id: Int, username: String }

pub type Refreshed = | Refreshed

## A category of feeds. Titles are unique: Miniflux refuses a second one.
pub type Category = { id: CategoryId, title: String }

## A subscribed feed, and whether Miniflux can read it.
pub type Subscription = {
  feedUrl: String,
  title: String,
  category: Category,
  ## Failed fetches since the last good one, and why the last one failed.
  parsingErrorCount: Int,
  parsingErrorMessage: String,
}

pub type Subscribed =
  | Added(feedId: Int)
  ## Such as "This feed already exists.", when Miniflux keeps the URL a
  ## redirect led to.
  | NotAdded(errorMessage: String)

@http(names: snake)
@auth(header: "X-Auth-Token", secret: "miniflux-key")
contract Miniflux {
  ## Who the key belongs to. Any answer means Miniflux is up; an error means
  ## it is paused, so this returns the error instead of waiting it out.
  @http("GET /v1/me")
  query me() -> Result(User, http.HttpError)

  ## A category's newest entries, read or not, published after a Unix time.
  @http("GET /v1/categories/:category/entries")
  query entries(
    category: CategoryId,
    publishedAfter: Int,
    limit: Int,
    status: List(String) = ["unread", "read"],
    order: String = "published_at",
    direction: String = "desc",
  ) -> Entries

  @http("GET /v1/categories")
  query categories() -> List(Category)

  @http("POST /v1/categories")
  command createCategory(title: String) -> Category

  @http("GET /v1/feeds")
  query feeds() -> List(Subscription)

  ## Subscribes to a feed, in a category.
  @http("POST /v1/feeds", status: { 201: Added, 400: NotAdded })
  command subscribe(feedUrl: String, categoryId: CategoryId) -> Subscribed

  ## Fetches every feed now, to catch up after a pause.
  @http("PUT /v1/feeds/refresh", status: { 204: Refreshed })
  command refresh() -> Refreshed
}
