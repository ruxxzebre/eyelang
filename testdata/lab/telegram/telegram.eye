# The Telegram Bot API, as the notifier uses it. Only the notifier holds the
# bot's token; other programs see Telegram through Notify.

pub type ChatId = | ChatId(Int)

pub type UpdateId = | UpdateId(Int)

pub type MessageRef = | MessageRef(Int)

pub type Button = { id: String, label: String }

pub type Update = {
  id: UpdateId,
  chat: ChatId,
  ## Whether the chat is a private one with a single person.
  private: Bool,
  ## Who sent it; None for channel posts.
  from: Option(ChatId),
  sentAt: Instant,
  body: UpdateBody,
}

pub type UpdateBody =
  | Command(name: String, args: String)
  | Pressed(message: MessageRef, button: String)
  | Text(String)

pub type Sending =
  | Posted(MessageRef)
  | TooFast(retryAfter: Duration)
  | Blocked

contract Telegram {
  ## Telegram takes no idempotency key, so a send whose reply is lost can't
  ## be looked up: it comes back Uncertain, and there is nothing to
  ## reconcile it with.
  command sendMessage(chat: ChatId, text: String, buttons: List(Button) = []) -> Sending

  command editMessage(chat: ChatId, message: MessageRef, text: String) -> Sending

  @key(id)
  event updates: Update on "chan.telegram.update"
}
