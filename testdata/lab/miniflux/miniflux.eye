# Miniflux, the feed reader the digest reads from. Lifecycle pauses it while
# the game server or a CI check needs its memory.

pub type CategoryId = | CategoryId(Int)

pub type Entry = {
  source: String,
  title: String,
  url: String,
  published: Instant,
  ## The entry's opening text, as plain text.
  text: String,
}

contract Miniflux {
  ## Whether Miniflux is up, rather than paused.
  query ready() -> Bool

  query entries(category: CategoryId, since: Instant, limit: Int) -> List(Entry)

  ## Fetches every feed now, to catch up after a pause.
  @key(required)
  command refresh() -> Refreshed
}

pub type Refreshed = | Refreshed
