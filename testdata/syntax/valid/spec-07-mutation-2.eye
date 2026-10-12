# SPECS.md section 7, example 2.
type Post = { links: List(String) }

fn links(feed: List(Post)) -> Set(String) {
  var seen: Set(String) = {}
  for post in feed {
    let fresh = post.links |> filter(fn l: !(seen |> contains(l)))   # a callback reads seen
    seen = seen + toSet(fresh)
  }
  let isNew = fn l: !(seen |> contains(l))    # error: a closure can't capture seen, a var
  seen
}
