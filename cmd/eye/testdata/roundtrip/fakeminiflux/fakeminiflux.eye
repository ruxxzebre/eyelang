# Miniflux served in Eyelang, for the round trip test of eye gen go and ts
# (cmd/eye/gen_roundtrip_test.go). The test copies it into a copy of the lab
# as module fakeminiflux, and gives the copy's Miniflux contract a port. Its
# answers show what each request carried.

import miniflux.{CategoryId, Category, Entries, Subscription}

impl miniflux.Miniflux {
  fn me() = Ok({ id: 1, username: "lab" })

  fn entries(category, publishedAfter, limit, status, order, direction) -> Entries = {
    total: limit,
    entries: [
      {
        title: "{category} {publishedAfter} {status |> join(",")} {order} {direction}",
        url: "https://news.example/{limit}",
        publishedAt: 2026-10-12 07:40 in "UTC",
        content: "<p>Hello</p>",
        feed: { title: "News" },
      },
    ],
  }

  fn categories() = [{ id: CategoryId(1), title: "News" }]

  fn createCategory(title) = { id: CategoryId(7), title }

  fn feeds() -> List(Subscription) = [
    {
      feedUrl: "https://news.example/feed",
      title: "News",
      category: { id: CategoryId(1), title: "News" },
      parsingErrorCount: 0,
      parsingErrorMessage: "",
    },
  ]

  fn subscribe(feedUrl, categoryId) =
    if feedUrl == "https://news.example/feed" then NotAdded(errorMessage: "This feed already exists.")
    else Added(feedId: 42)

  fn refresh() = Refreshed
}
