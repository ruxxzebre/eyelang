# A contract that sends every type of the lab's contracts back as it came,
# for the round trip test of eye gen go and ts (cmd/eye/gen_roundtrip_test.go).
# The test copies it into a copy of the lab as module echo. Its names are
# camelCase on the wire, where Notify's and Miniflux's are snake_case.

import home.{UserId}
import http
import notify.{Priority, Format, MessageId, Delivery, Refusal, Choice, QuestionId, Asked, Answered}
import miniflux.{CategoryId, Feed, Entry, Entries, User, Refreshed, Category, Created, Subscription, Subscribed}
import telegram.{ChatId, UpdateId, MessageRef, Button, Update, UpdateBody, Sending}
import authentik.{Person}
import flaresolverr.{Proxy, Solution, Solved}
import irishrail.{TrainCode, StopId, Movement}
import netbird.{PeerUserId, PeerUser, Pending, Change}

## What a request with arguments in its path and query carried.
pub type Ids = {
  chat: ChatId,
  user: UserId,
  priority: Priority,
  tags: List(String),
  limit: Option(Int),
  at: Instant,
}

## The arguments of a call that left some out, with their defaults.
pub type Defaults = {
  given: String,
  priority: Priority,
  tags: List(String),
  note: Option(String),
  count: Int,
}

pub type Grade =
  | Good(score: Int)
  | Poor(score: Int, reason: String)

pub type Cleared = | Cleared(id: String, by: String, what: String, key: String)

@port(3990)
@auth(bearer: "echo-token")
contract Echo {
  @http("POST /priority")
  query priority(value: Priority) -> Priority

  @http("POST /format")
  query format(value: Format) -> Format

  @http("POST /message-id")
  query messageId(value: MessageId) -> MessageId

  @http("POST /delivery")
  query delivery(value: Delivery) -> Delivery

  @http("POST /refusal")
  query refusal(value: Refusal) -> Refusal

  @http("POST /choice")
  query choice(value: Choice) -> Choice

  @http("POST /question-id")
  query questionId(value: QuestionId) -> QuestionId

  @http("POST /asked")
  query asked(value: Asked) -> Asked

  @http("POST /answered")
  query answered(value: Answered) -> Answered

  @http("POST /user-id")
  query userId(value: UserId) -> UserId

  @http("POST /category-id")
  query categoryId(value: CategoryId) -> CategoryId

  @http("POST /feed")
  query feed(value: Feed) -> Feed

  @http("POST /entry")
  query entry(value: Entry) -> Entry

  @http("POST /entries")
  query entries(value: Entries) -> Entries

  @http("POST /user")
  query user(value: User) -> User

  @http("POST /refreshed")
  query refreshed(value: Refreshed) -> Refreshed

  @http("POST /category")
  query category(value: Category) -> Category

  @http("POST /created")
  query created(value: Created) -> Created

  @http("POST /subscription")
  query subscription(value: Subscription) -> Subscription

  @http("POST /subscribed")
  query subscribed(value: Subscribed) -> Subscribed

  @http("POST /chat-id")
  query chatId(value: ChatId) -> ChatId

  @http("POST /update-id")
  query updateId(value: UpdateId) -> UpdateId

  @http("POST /message-ref")
  query messageRef(value: MessageRef) -> MessageRef

  @http("POST /button")
  query button(value: Button) -> Button

  @http("POST /update")
  query update(value: Update) -> Update

  @http("POST /update-body")
  query updateBody(value: UpdateBody) -> UpdateBody

  @http("POST /sending")
  query sending(value: Sending) -> Sending

  @http("POST /person")
  query person(value: Person) -> Person

  @http("POST /proxy")
  query proxy(value: Proxy) -> Proxy

  @http("POST /solution")
  query solution(value: Solution) -> Solution

  @http("POST /solved")
  query solved(value: Solved) -> Solved

  @http("POST /train-code")
  query trainCode(value: TrainCode) -> TrainCode

  @http("POST /stop-id")
  query stopId(value: StopId) -> StopId

  @http("POST /movement")
  query movement(value: Movement) -> Movement

  @http("POST /peer-user-id")
  query peerUserId(value: PeerUserId) -> PeerUserId

  @http("POST /peer-user")
  query peerUser(value: PeerUser) -> PeerUser

  @http("POST /pending")
  query pending(value: Pending) -> Pending

  @http("POST /change")
  query change(value: Change) -> Change

  @http("POST /version")
  query version(value: Version) -> Version

  @http("POST /approval")
  query approval(value: Approval) -> Approval

  ## Arguments in the path and the query, as text.
  @http("GET /ids/:chat")
  query ids(chat: ChatId, user: UserId, priority: Priority, tags: List(String), limit: Option(Int), at: Instant) -> Ids

  ## Arguments left out take their defaults.
  @http("POST /defaults")
  query defaults(given: String, priority: Priority = High, tags: List(String) = ["a", "b"], note: Option(String) = None, count: Int = 3) -> Defaults

  ## A failure the caller handles: 200 is Ok, anything else Err.
  @http("GET /page")
  query page(status: Int) -> Result(String, http.HttpError)

  ## Results whose status `status:` names.
  @http("POST /grade", status: { 201: Good, 422: Poor })
  command grade(score: Int) -> Grade

  ## A fresh and destructive command: basedOn 1 is Stale, 2 Expired.
  @http("POST /shelves/:id/clear")
  @fresh
  @destructive
  @key(required)
  command clear(id: String) -> Cleared

  ## A command whose impl faults, which its caller sees as Uncertain.
  @http("POST /fail")
  @key(required)
  command fail(why: String) -> Grade
}
