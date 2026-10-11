# The team's chat. Posting keeps what each key did, so posting again with the
# same key is how a lost reply is settled.

pub type Post =
  | Posted(id: String)
  | Refused(error: String)

pub type Message = { by: String, text: String }

contract Chat {
  ## Posts a message to the team's channel.
  @key(required)
  @reconcile(post)
  command post(text: String) -> Post

  ## The channel's latest messages.
  query history(limit: Int) -> List(Message)
}
