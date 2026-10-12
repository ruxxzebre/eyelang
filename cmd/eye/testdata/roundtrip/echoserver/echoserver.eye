# Serves the Echo contract: every value comes back as it came. For the round
# trip test of eye gen go and ts (cmd/eye/gen_roundtrip_test.go).

import echo.{Cleared}
import http

impl echo.Echo {
  fn priority(value) = value
  fn format(value) = value
  fn messageId(value) = value
  fn delivery(value) = value
  fn refusal(value) = value
  fn choice(value) = value
  fn questionId(value) = value
  fn asked(value) = value
  fn answered(value) = value
  fn userId(value) = value
  fn categoryId(value) = value
  fn feed(value) = value
  fn entry(value) = value
  fn entries(value) = value
  fn user(value) = value
  fn refreshed(value) = value
  fn category(value) = value
  fn created(value) = value
  fn subscription(value) = value
  fn subscribed(value) = value
  fn chatId(value) = value
  fn updateId(value) = value
  fn messageRef(value) = value
  fn button(value) = value
  fn update(value) = value
  fn updateBody(value) = value
  fn sending(value) = value
  fn person(value) = value
  fn proxy(value) = value
  fn solution(value) = value
  fn solved(value) = value
  fn trainCode(value) = value
  fn stopId(value) = value
  fn movement(value) = value
  fn peerUserId(value) = value
  fn peerUser(value) = value
  fn pending(value) = value
  fn change(value) = value
  fn version(value) = value
  fn approval(value) = value

  fn ids(chat, user, priority, tags, limit, at) = { chat, user, priority, tags, limit, at }

  fn defaults(given, priority, tags, note, count) = { given, priority, tags, note, count }

  fn page(status) =
    if status == 200 then Ok("the page") else Err(http.BadStatus(code: status, body: "paused"))

  fn grade(score) = if score >= 50 then Good(score:) else Poor(score:, reason: "below 50")

  fn clear(id, basedOn, approval, key) {
    if basedOn == 1 { return Stale }
    if basedOn == 2 { return Expired }
    Cleared(id:, by: approval.by, what: approval.what, key: "{key}")
  }

  fn fail(why, key) = unreachable("{why} ({key})")
}
