# SPECS.md section 19, example 1.
import ai

## How one morning's news is summarised.
type Digest = {
  ## One sentence, at most 25 words.
  headline: String,
  topics: List(Topic),
  urgency: Urgency,
}

type Topic = { name: String, items: List(String) }
type Urgency = | Calm | Busy | Breaking(reason: String)

@budget(usd: 0.50)
job summarise(items: List(Item)) -> Result(Digest, AiError) {
  ai.generate!(
    model: "claude-sonnet-5-5",
    system: """
      You summarise Irish news for a morning digest.
      Use only facts in the input. Never guess dates, numbers or rules.
      """,
    prompt: items |> map(fn i: "- {i.title}") |> join("\n"),
  )
}
