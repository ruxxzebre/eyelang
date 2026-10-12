# The team's chat. Posting keeps what each key did, so posting again with the
# same key is how a lost reply is settled.

## What the chat did with a message.
pub type Post =
  | Posted(id: String)
  ## The channel is archived, or the bot was removed from it.
  | Refused(error: String)

# region contract
contract Chat {
  ## Posts a message to a channel. Each key is posted at most once.
  @key(required)
  @reconcile(post)
  command post(channel: String, text: String) -> Post
}
# endregion contract
