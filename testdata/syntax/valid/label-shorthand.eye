# `key:` alone passes the variable named key: `send(key:)` is `send(key: key)`.

fn deliver(article: Article, priority: Priority, key: Key) {
  notify.send!(
    title: article.title,
    priority:,
    format: Markdown,
    key:,
  )
  log.info(key:, at: now())
}
