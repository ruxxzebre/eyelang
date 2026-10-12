---
title: "AI models and agents"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# AI models and agents

Programs that call language models, and jobs that run coding agents, use the pieces the rest of the
language already has: types, effects, `!` steps and recordings. The standard library adds two
modules, `ai` and `agents`. Syntax that helps here and elsewhere is welcome: `##` doc comments
describe schemas and tools as well as documentation, and prompts are written as multi-line
strings ([section 4](/reference/language/lexical-structure)).

**Typed model calls.** A call names the type it wants back. The compiler derives a JSON Schema from
that type, the provider is asked for structured output matching it, and the reply is checked and
decoded into an Eyelang value. Code never sees unchecked text.

```eye
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
job summarise(items: List(Item)) -> Result(Digest, ai.AiError) {
  ai.generate!(
    model: "claude-sonnet-5-5",
    system: """
      You summarise Irish news for a morning digest.
      Use only facts in the input. Never guess dates, numbers or rules.
      """,
    prompt: items |> map(fn i: "- {i.title}") |> join("\n"),
  )
}
```

- **Schemas come from types.** Records, variants, lists, maps with string keys, `Option`, tuples,
  numbers, strings, booleans, dates and times have a derived `Schema` trait. The schema describes
  the JSON shape of [section 17](/reference/language/contracts-talking-to-other-services): a record becomes an object with every field required (`Option`
  fields may be null), a type whose variants have no fields an enum of strings, and any other
  variant a tagged union, `{"kind": "Breaking", "reason": "…"}`; `##` doc comments become
  descriptions. Strict schemas, OpenAI's and Anthropic's alike, need `additionalProperties:
  false` on every object and have no fixed-length arrays, so a model is asked for a map as a
  list of `{"key": …, "value": …}` entries and for a tuple as an object with fields `value1`,
  `value2`, …; reading a reply takes either shape. A type that holds a `Secret`, a value of a
  program's `extern type` or a function has no `Schema`, so it can't be asked for.
- **The expected type picks the schema.** `ai.generate` returns `Result(a, AiError) where a:
  Schema`, so the type comes from where the result goes: here, the job's return type. When nothing
  says, write it: `let d: Digest = ai.generate!(…)?`.
- **Replies are checked.** If a reply doesn't match the schema, the call asks once more with the
  problems listed, and then returns `Err(Malformed(reply, problems))`.
- **Providers.** `ai.Provider` is `Anthropic` (the Messages API), `OpenAI` (chat completions) or
  `Gateway`, an OpenAI-compatible server such as the lab's, which serves every model. A call has
  the effect `net Ai.generate`, which `uses {net}` covers too (`Ai` is the `ai` module's contract
  for the providers), and each provider's address and key come from the environment and `secret`:
  `ANTHROPIC_URL` and `secret("anthropic-api-key")`, `OPENAI_URL` and `secret("openai-api-key")`,
  `AI_URL` and `secret("ai-key")`. `provider: Some(ai.Anthropic)` names one; without it, a gateway
  answers every call when `AI_URL` is set, since a deployment that has one routes through it, and
  otherwise the model's name decides: `claude-…` is Anthropic, `gpt-…` and `o1`, `o3`… OpenAI.
- **In a job, every model call is a `!` step.** Its reply is recorded, so a resumed run neither calls
  the model again nor pays twice, and it sees the same answer.
- **Tests replay recordings.** `with fakes(ai, from: "recordings/")` makes model-backed tests
  deterministic and free, and a recording that no longer matches the schema fails its test.
  `eye test --live ai` runs them against the real models, and `--record` writes their replies
  as the recordings ([section 20](/reference/language/time-secrets-tests)).
- **Budgets.** `@budget(usd: 5, tokens: 2_000_000)` on a job or function works like `@memory`: a call
  that would go over it isn't sent, and the run ends as `Failed(OverBudget)`. A job's budget is for
  its run, a function's for each call of it, and a call counts against every budget it runs
  under. Each reply's tokens, and their price, are recorded with its step, so a resumed run counts
  them again and a restart doesn't reset the budget. A call is sent only if the budget leaves
  room for its prompt and at least 64 tokens of reply, and the reply is limited to what is left;
  a reply cut off there ends the run as `Failed(OverBudget)` too. Prices are known for Anthropic's
  models, and a deployment gives others in `AI_PRICES` (`"gpt-5.5=1.25/10"`: dollars per million
  tokens in and out); a provider that reports what a call cost is taken at its word. A budget in
  dollars for a model whose price isn't known isn't kept quietly by tokens instead: the call
  stops with a fault, `Unreachable`, that says to give the price. Recordings may say what a reply spent
  (`"usage": {"input": 900, "output": 100}`), and tests count it.

**Tools are functions.** Eyelang functions passed as `tools:` become the model's tools. A tool's
parameter types give its input schema and its `##` comment its description, and the runtime runs
the function when the model calls it and sends back what it returns.

```eye
import ai

## Find open issues that mention a phrase.
fn searchIssues(phrase: String, limit: Int = 10) -> List(Issue) uses {net GitHub.search} {
  github.search(phrase, limit)
}

## Add a label to an issue. False if there is no such label, or if GitHub's
## reply was lost.
@destructive
fn label(issue: IssueId, name: String) -> Bool uses {net GitHub.label} {
  match github.label(issue, name, approval) {
    Added => true
    NoSuchLabel | Uncertain(_) => false
  }
}

type Triage = { duplicateOf: Option(IssueId), labels: List(String) }

job triage(report: String) -> Result(Triage, ai.AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label],
    approve: fn call: approval!(call, from: maintainer, within: 1 day))
}
```

- **A tool is a function declared with `fn`, named where it is passed.** `tools:` is a
  `List(ai.Tool)`, and a named function converts to an `ai.Tool` where one is expected, as a
  value converts to `dyn Trait` ([section 11](/reference/language/traits)), so one list holds functions of different types.
  Its parameters and its result must have `Schema`. The parameters give the input schema, one
  property each, required unless it has a default, and the `##` comment the description. A
  lambda, a function passed in, a generic function, an `extern fn`, a contract's operation and a
  built-in don't convert: a tool needs a name, a description and one schema. A `Tool` has no
  `Save` and no `Schema`, so it can't be kept in an entity or across a `!`.
- **What a model may do shows up in the signature.** A model call's own effect is
  `net Ai.generate`, and naming a function as a tool counts its effects where it is named, as for
  any function named as a value ([section 12](/reference/language/effects-and-signatures)). So `triage` uses `net Ai.generate`,
  `net GitHub.search` and `net GitHub.label`; a public function that passes `label` lists
  `net GitHub.label` in its `uses`, and adding a tool shows up in review. A tool runs inside the
  model call, so it can't wait.
- **The runtime holds the conversation.** It sends the tools with the call, in each provider's
  own format, and when a reply calls tools, it decodes each call's input into the parameters'
  types, runs the function and sends back its result as JSON ([section 17](/reference/language/contracts-talking-to-other-services)), until the model
  answers without calling one. That answer is decoded into the expected type, as above. A call
  whose input doesn't decode, or that names no tool, goes back to the model as an error, and no
  function runs. After 20 replies that call tools, the call gives up with `ProviderError`.
- **In a job, each tool call is a step.** In `ai.generate!(…)` every reply of the model and every
  tool's result is recorded as a step of its own, so a run that restarts in the middle replays
  the conversation so far: it doesn't ask the model again, and no tool runs twice. `eye runs`
  shows them as `ai.generate: reply` and `tool: searchIssues`.
- **A `@destructive` tool needs a person for each call.** `@destructive` on a function adds a
  required `approval: Approval` parameter after those it declares, as on a command ([section 17](/reference/language/contracts-talking-to-other-services)),
  so code calls it only after a person said yes, and its body can pass the approval on, as
  `label` does. The parameter isn't in the tool's schema. When the model calls the tool, the
  runtime calls `approve:` with the call written out, `label(issue: IssueId("8"), name: "dup")`,
  and runs the tool with the `Approval` it gets. `approve:` is usually
  `fn call: approval!(call, from: someone, within: …)`, so the request goes to the person it
  names, and only they can answer it ([section 15](/reference/language/jobs-and-waiting)); a test answers with `approve("label(")`. On
  `Declined` or `Expired` the tool doesn't run, and the model is told that the call was declined.
  Without `approve:` every such call is declined, and a call whose own `tools:` list names a
  `@destructive` tool must give `approve:`.
- **Recordings hold tool calls too.** In a recording, an exchange whose reply calls tools has
  `"calls": [{"tool": "searchIssues", "input": {"phrase": "crash"}}]` in place of `"result"`.

**Agents are sessions owned by jobs.** A coding agent such as Claude Code or OpenAI Codex works for
minutes or hours, asks for permission and can be interrupted. `agents` drives them through their
SDKs as sessions that a job owns. The job holds only the session's ID, so it waits without holding
memory and picks the session up again after a restart.

```eye
import agents.{claude, codex}

type Review = { verdict: Verdict, findings: List(Finding) }
type Verdict = | Approve | RequestChanges(reason: String)
type Finding = { file: String, line: Int, message: String }

@on(event(github.pullRequestOpened))
@budget(usd: 5)
@deadline(2 h)
job review(pr: PullRequest) -> Result(Review, agents.AgentError) {
  let session = claude.start!(
    prompt: "Review {pr.url}. Report real bugs only.",
    workdir: checkout!(pr.head),
    tools: [readFiles, runTests],
  )
  let review: Review = session.result!()?
  if review.findings != [] {
    let fixed: String = codex.start!(prompt: "Fix these: {review.findings}",
      workdir: checkout!(pr.head), tools: [editFiles, runTests]).result!()?
    log.info("codex: {fixed}")
  }
  Ok(review)
}
```

- `start!` begins a session, `send!` adds a message, and `result!()` waits for the final answer,
  decoded into the expected type through the harness's structured output.
- `tools:` lists what the agent may use. Eyelang functions are served to the agent over MCP, and
  their effects become the job's, as with model calls. Permission requests from the agent arrive
  as events; code can answer them, or hand them to a person with `approval!`.
- Several agents run side by side as child runs, joined with `allDone!` or `firstDone!`.
