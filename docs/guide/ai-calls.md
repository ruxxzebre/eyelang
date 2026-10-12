---
title: AI calls
---

# AI calls

A language model answers with text. Code that needs a category, a date or a yes-or-no has to parse
that text, and the parsing breaks on the reply nobody expected: a category that doesn't exist, a
field left out, a sentence before the JSON. Long-running programs add two more problems. A program
that restarts in the middle of its work calls the model again and pays again, and may get a
different answer. And tests that call a real model are slow, cost money, and don't give the same
result twice.

In Eyelang a model call names the **type** it wants back. The compiler turns the type into a JSON
Schema, the model is asked for output that matches it, and the reply is checked and decoded into a
value of that type. Your code never sees unchecked text. In a job the call is a recorded step, and
in tests it replays a recording.

The code on this page is a [real program](https://github.com/ruxxzebre/eyelang/tree/dev/docs/guide/code/ai-calls):
a support desk where a model sorts each incoming message. Its tests run with
`eye test docs/guide/code/ai-calls`.

[Open it in the playground](/playground#example=guide/ai-calls) to run it, test it and change it in your browser.

## Ask for a type

Start with the type you want. Write it as you would any other type, with a `##` comment on anything
the model needs to understand:

<<< ./code/ai-calls/main.eye#types

Then ask for it with `ai.generate`:

<<< ./code/ai-calls/main.eye#ask

`ai.generate` returns `Result(a, ai.AiError)`, and `a` is whatever type the result is used as. Here
`sortMessage` returns `Result(Triage, ai.AiError)`, so the model is asked for a `Triage`. When
nothing says what the type is, name it:

```eye
let t: Result(Triage, ai.AiError) = ai.generate!(model: "claude-sonnet-5-5", prompt: text)
```

Without a type, the call stops with a fault:

```text
main.eye:5:11: ai.generate needs to know the type to ask for: write `let x: T = ai.generate!(…)`, or return it from a function that says its type
```

Write instructions for the whole task as a multi-line string, as `instructions` is above, and put
what one field means in that field's `##` comment.

## The type becomes a schema

This is the schema the model gateway received for `Triage`, taken from a real run:

```json
{
  "type": "object",
  "description": "How a support message is sorted.",
  "properties": {
    "category": { "type": "string", "enum": ["Bug", "Question", "Billing"] },
    "urgency": {
      "anyOf": [
        {
          "type": "object",
          "properties": { "kind": { "type": "string", "const": "Normal" } },
          "required": ["kind"],
          "additionalProperties": false
        },
        {
          "type": "object",
          "description": "Someone can't work, or is losing money, right now.",
          "properties": {
            "kind": { "type": "string", "const": "Urgent" },
            "reason": { "type": "string" }
          },
          "required": ["kind", "reason"],
          "additionalProperties": false
        }
      ]
    },
    "summary": {
      "type": "string",
      "description": "One sentence, at most 20 words, saying what the customer wants."
    },
    "draft": {
      "anyOf": [{ "type": "string" }, { "type": "null" }],
      "description": "A reply to send as it is, only when the answer is simple and certain."
    }
  },
  "required": ["category", "draft", "summary", "urgency"],
  "additionalProperties": false
}
```

The schema describes the same JSON shape that values have everywhere else (see
[Values as JSON](/guide/contracts#values-as-json)):

| Eyelang | Schema |
|---|---|
| A record | An object; every field is required, and no other fields are allowed |
| `Option(T)` | `T` or `null` |
| A type whose variants have no fields, such as `Category` | A string from a fixed list |
| Any other variant type, such as `Urgency` | One of several objects, each tagged with its `"kind"` |
| `List(T)`, `Set(T)` | An array |
| `String`, `Int`, `Float`, `Bool` | A string, an integer, a number, a boolean |
| `Instant`, `Date` | A string with the format `date-time` or `date` |
| `Duration` | An integer: milliseconds |
| A type that wraps one value | The schema of that value |

The `##` comments become the schema's `description`s: the type's comment, each field's, and each
variant's. The comment that tells a person what `summary` holds also tells the model. Write it as
an instruction you would give a careful colleague: what goes there, how long it is, when to leave
it empty. See [Doc comments](/guide/doc-comments).

A type that holds a `Secret`, a `Handle` or a function can't be asked for, and the checker says so.
If you ask for something that isn't an object, such as a `List(String)`, it is wrapped in an object
with one field, `value`, since providers want an object at the top.

Strict schemas have no maps and no fixed-length lists, so a map is asked for as a list of
`{"key": …, "value": …}` objects, and a tuple as an object with fields `value1`, `value2`, and so
on. A time of day is text such as `"14:30"`. The reply is read back into the map, tuple or time
your type says.

A type that refers to itself, such as a tree, has no schema, since strict schemas can't be
recursive; nor have `()`, `Char`, `Bytes` and the fixed-width integers. `eye check` says so at the
call. Ask for a tree as a flat list whose items name their parent.

## Replies are checked

A reply is decoded into the type you asked for, or it isn't a value at all. When a reply doesn't
match, the call asks once more, and tells the model what was wrong. This is the second request from
the same run, after a reply with a category that doesn't exist:

```text
That reply doesn't match the schema: category: expected one of "Bug", "Question", "Billing", found "Shipping". Reply again with JSON that matches it.
```

If the second reply doesn't match either, the call returns `Err(Malformed(reply, problems))`. The
job decides what that means for the program. Here a person sorts the ticket:

<<< ./code/ai-calls/main.eye#job

<<< ./code/ai-calls/main.eye#test-garbled

## When the call fails

`ai.AiError` has three cases, and a `match` on it must handle each, or say `_`:

| Case | When |
|---|---|
| `Malformed(reply, problems)` | Two replies didn't match the type |
| `ProviderError(status, message)` | The provider answered with an error, couldn't be reached (status 0), or the reply didn't finish |
| `Declined(reason)` | The model declined to answer |

A provider that can't be reached, or answers 429 or a 5xx, is tried again for about a minute while
the run waits, as a contract's query is; then the call is `ProviderError`, its message saying why,
as "api.anthropic.com refused the connection, 8 times over a minute". A request that went out and
had no answer isn't sent again, since the provider may have done the work and charged for it. Other
failures are not retried, apart from the one extra request for a malformed reply. Decide in the
code what a failure costs. In the [approvals tutorial](/tutorial/approvals) a failed summary costs
the summary, not the request; here it sends the ticket to a person.

## In a job, a call is a recorded step

`sortMessage` waits for the model, so it is called with `!`, and in a job that makes it a step. Its
result is saved before the job goes on. If the program restarts after the step, the run carries on
with the same answer: the model isn't asked again, isn't paid again, and can't change its mind.

`eye runs` shows the saved reply as the run's first step:

```text
$ eye runs . 1
#1 triage("t-1", "I was charged twice for order 1182")  done  [2 steps, 561 B]
    1. step → Ok({ category: Billing, urgency: Urgent(reason: "the customer is losing money"), summary: "The cust…
    2. Ticket(t-1).sort
```

Outside a job, for example in a function that `main` calls, `ai.generate` is called without `!`,
and nothing is recorded.

## Tests replay recorded replies

`with fakes(ai, from: "recordings/")` answers model calls from `recordings/ai.json` instead of a
model:

<<< ./code/ai-calls/recordings/ai.json

<<< ./code/ai-calls/main.eye#test-sorted

- Each `result` is a reply: the value asked for, in its JSON shape.
- Replies are used in order, one per request. A reply that doesn't match counts as a malformed
  reply, so the call asks again and uses the next one. That is why the recording for the garbled
  test holds two replies.
- A call with no reply left stops the run, and its test fails.
- Keep each scenario in its own directory, such as `recordings/garbled/`.

The tests are free, fast, and the same on every run. `eye test --live ai` runs them against the
real model instead, and skips them when no provider's address or key is set; `--live ai --record`
also writes the model's replies as `ai.json`, without the key, for the next `eye test` to replay.
You can also write the value you want to test against by hand.

::: warning
A test without `fakes(ai, …)` calls the real provider if its address or key is set in the environment.
:::

See [Testing with time and recordings](/guide/testing) for more on recordings and test clocks.

## Choosing the provider

Three providers answer model calls, each with structured output, and `ai.Provider` names them:

| Provider | Protocol | Address | Key |
|---|---|---|---|
| `Anthropic` | the Messages API | `ANTHROPIC_URL`, or `https://api.anthropic.com` | `secret("anthropic-api-key")`: `ANTHROPIC_API_KEY` |
| `OpenAI` | chat completions | `OPENAI_URL`, or `https://api.openai.com/v1` | `secret("openai-api-key")`: `OPENAI_API_KEY` |
| `Gateway` | OpenAI-compatible chat completions, for any model | `AI_URL`, including the version: `http://gateway:8317/v1` | `secret("ai-key")`, if it needs one: `AI_KEY` |

Which one answers a call:

1. The one `provider:` names: `ai.generate!(model: "claude-opus-5-5", prompt: text, provider: Some(ai.Anthropic))`.
2. Otherwise, the gateway, when `AI_URL` is set. A deployment that has a gateway sends every model
   through it, and the `model:` argument goes to it as it is, so use the names your gateway knows.
3. Otherwise, the model's name: `claude-…` goes to Anthropic, and `gpt-…`, `o1`, `o3` and the like
   to OpenAI. Any other name needs a gateway, and the call stops its run without one:

```text
main.eye:12:3: job sort: ai.generate needs a model gateway for house-model: set AI_URL to its base URL, such as http://gateway:8317/v1, or name the provider with provider:
```

`effort:` asks the model to reason more or less before it answers: `Some(Low)`, `Some(Medium)`
or `Some(High)`. Anthropic gets it as `output_config.effort`, and
OpenAI and a gateway as `reasoning_effort`; without it the provider's default holds. More effort
costs more tokens and time.

## Budgets

`@budget(usd:, tokens:)` on a job limits what the model calls of one run spend; on a function, what
one call of it spends. Every call counts against each budget it runs under.

```eye
@budget(usd: 0.50)
job summarise(text: String) -> Result(Summary, ai.AiError) {
  ai.generate!(model: "claude-sonnet-5-5", prompt: text)
}
```

- A call is sent only if the budget leaves room for its prompt and at least 64 tokens of reply,
  and the reply is limited to what is left. Otherwise the call isn't sent, and the run ends as
  `Failed(OverBudget)`; a reply cut off at the limit ends it the same way. A caller that must
  report it starts the work as a child run and joins it, as the lab's digest does.
- What each reply spent is recorded with its step, so a run that resumes after a restart counts it
  again: a restart doesn't refill the budget.
- Dollars need prices. Anthropic's models are known; for others, the deployment gives the price in
  `AI_PRICES`, as dollars per million tokens in and out: `AI_PRICES="gpt-5.5=1.25/10,house=0/0"`.
  A gateway that reports what a call cost (`usage.cost`) is taken at its word. A budget in dollars
  for a model with no known price isn't quietly kept by tokens instead; the call stops:

```text
main.eye:12:3: job write: @budget(usd: 0.5) can't be kept, since the price of house-model isn't known: give it in AI_PRICES, such as AI_PRICES="house-model=3/15" for $3 per million tokens in and $15 out, or limit the budget by tokens
```

- In tests, a recording may say what a reply spent, and the budget counts it:
  `{"op": "generate", "result": {…}, "usage": {"input": 900, "output": 100}}`.

See [Running in production](/guide/running#configuration) for the other settings.

## Tools

Functions passed as `tools:` are the model's tools. The model reads each one's name, `##` comment
and parameters, and when it calls one, the function runs and its result goes back to the model,
until the model answers with the type asked for.

```eye
## Finds open tickets whose text mentions a phrase.
fn searchTickets(phrase: String, limit: Int = 10) -> List(TicketId) uses {entity Ticket} {
  Tickets("open").ids |> filter(fn id: Ticket(id).read().text |> contains(phrase)) |> take(limit)
}

## Closes a ticket as a duplicate of another.
@destructive
fn closeAsDuplicate(id: TicketId, of: TicketId) uses {entity Ticket} {
  Ticket(id).closeAsDuplicate(of, approval)
}

job dedupe(id: TicketId, text: String) -> Result(Triage, ai.AiError) {
  ai.generate!(model: "claude-sonnet-5-5", prompt: text, tools: [searchTickets, closeAsDuplicate],
    approve: fn call: approval!(call, from: "support-lead", within: 1 day))
}
```

- A tool is a function declared with `fn`, named in the list. Its parameters and its result must
  be types a model can be asked for; a parameter with a default may be left out.
- The tools' effects are the caller's. A public function that passes `searchTickets` lists
  `entity Ticket` in its `uses`, next to the call's own `net Ai.generate`.
- In a job, each reply and each tool's result is a step, so a run that restarts in the middle
  neither asks the model again nor runs a tool twice. `eye runs` shows them as
  `ai.generate: reply` and `tool: searchTickets`.
- A `@destructive` function takes an `approval: Approval` after its declared parameters. When the
  model calls it, `approve:` asks a person about that one call, written out as
  `closeAsDuplicate(id: …, of: …)`, and the tool runs only if they approve; otherwise the model
  hears that the call was declined. A test answers with `approve("closeAsDuplicate(")`.
- In a recording, a reply that calls tools is
  `{"op": "generate", "calls": [{"tool": "searchTickets", "input": {"phrase": "refund"}}]}`.

## Not yet

- **Agents.** Running coding agents with the `agents` module is designed (see
  [AI models and agents](/reference/language/ai-models-and-agents)) but not built
  ([#60](https://github.com/ruxxzebre/eyelang/issues/60)).

## See also

- [AI models and agents](/reference/language/ai-models-and-agents), the exact rules.
- [Doc comments](/guide/doc-comments): writing the descriptions a model reads.
- [Talking to other services](/guide/contracts): the JSON shape of values.
- [All together: approvals](/tutorial/approvals): a model call in a larger program.
- API: [ai](/reference/api/std/ai).
