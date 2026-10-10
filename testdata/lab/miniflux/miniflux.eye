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

  ## Fetches every feed now, to catch up after a pause.
  @http("PUT /v1/feeds/refresh", status: { 204: Refreshed })
  command refresh() -> Refreshed
}
