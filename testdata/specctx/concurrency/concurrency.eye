# What SPECS.md §13's examples of tasks assume (context=concurrency): pages
# to fetch, a user and the services that know about them, two mirrors of a
# file, and feeds to read one by one.

import http
import users
import orders
import mirrorA
import mirrorB

let urls = ["https://example.com/a", "https://example.com/b"]

let id = users.UserId("ann")

let path = "/releases/latest.tar.gz"

let feeds = ["https://example.com/feed.xml", "https://example.org/rss"]

fn fetch(url: String) -> String uses {net} =
  match http.get(url) {
    Ok(r) => r.body
    Err(_) => ""
  }

fn handle(page: String) {
  log.info("read {size(page)} characters")
}
