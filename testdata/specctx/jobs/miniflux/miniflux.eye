# The feed reader the digest reads unread items from.

pub type Item = { title: String, url: String }

pub type FeedError = | Unauthorized | Unavailable(String)

contract Miniflux {
  query unread(category: String, since: Date) -> Result(List(Item), FeedError)
}
