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
```

- **Schemas come from types.** Records, variants, lists, maps with string keys, `Option`, tuples,
  numbers, strings, booleans, dates and times have a derived `Schema` trait. The schema describes
  the JSON shape of [section 17](/reference/language/contracts-talking-to-other-services): a record becomes an object with every field required (`Option`
  fields may be null), a type whose variants have no fields an enum of strings, and any other
  variant a tagged union, `{"kind": "Breaking", "reason": "…"}`; `##` doc comments become
  descriptions. A type that holds a `Secret`, a `Handle` or a function has no `Schema`, so it can't
  be asked for.
- **The expected type picks the schema.** `ai.generate` returns `Result(a, AiError) where a:
  Schema`, so the type comes from where the result goes: here, the job's return type. When nothing
  says, write it: `let d: Digest = ai.generate!(…)?`.
- **Replies are checked.** If a reply doesn't match the schema, the call asks once more with the
  problems listed, and then returns `Err(Malformed(reply, problems))`.
- **Providers are contracts.** `ai` declares contracts for Anthropic, OpenAI and OpenAI-compatible
  servers. A call has the effect `net` on its provider, the key comes from `secret`, and the
  provider follows from the model's name unless `provider:` says otherwise. The reference
  interpreter speaks the OpenAI-compatible chat completions protocol with structured output, which
  gateways such as the lab's offer for every model: `AI_URL` is the gateway's base URL, and
  `secret("ai-key")` its key, if it needs one.
- **In a job, every model call is a `!` step.** Its reply is recorded, so a resumed run neither calls
  the model again nor pays twice, and it sees the same answer.
- **Tests replay recordings.** `with fakes(ai, from: "recordings/")` makes model-backed tests
  deterministic and free, and a recording that no longer matches the schema fails its test.
  `eye test --live` runs them against the real models.
- **Budgets.** `@budget(usd: 5, tokens: 2_000_000)` on a job or function works like `@memory`: a call
  that would go over it isn't sent, and the run ends as `Failed(OverBudget)`.

**Tools are functions.** Eyelang functions passed as `tools:` become the model's tools. A tool's
parameter types give its input schema and its `##` comment its description, and the runtime runs
the function when the model calls it.

```eye
## Find open issues that mention a phrase.
fn searchIssues(phrase: String, limit: Int = 10) -> List(Issue) uses {net GitHub.search} {
  github.search(phrase, limit)
}

## Add a label to an issue.
@destructive
fn label(issue: IssueId, name: String) uses {net GitHub.label} {
  github.label(issue, name)
}

type Triage = { duplicateOf: Option(IssueId), labels: List(String) }

job triage(report: String) -> Result(Triage, AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label])
}
```

- **What a model may do shows up in the signature.** The tools' effects are the call's effects, so
  `triage` uses `net GitHub.search` and `net GitHub.label`, and adding a tool shows up in review.
- **Some calls need a person.** When the model calls a `@destructive` tool, the job pauses, as at
  `approval!`, until someone approves that one call.
- In a job, each tool call is recorded like any other step.

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
job review(pr: PullRequest) -> Result(Review, AgentError) {
  let session = claude.start!(
    prompt: "Review {pr.url}. Report real bugs only.",
    workdir: checkout!(pr.head),
    tools: [readFiles, runTests],
  )
  let review: Review = session.result!()?
  if review.findings != [] {
    codex.start!(prompt: "Fix these: {review.findings}", workdir: checkout!(pr.head),
      tools: [editFiles, runTests]).result!()?
  }
  Ok(review)
}
```

- `start!` begins a session, `send!` adds a message, and `result!()` waits for the final answer,
  decoded into the expected type through the harness's structured output.
- `tools:` lists what the agent may use. Eyelang functions are served to the agent over MCP, and
  their effects become the job's, as with model calls. Permission requests from the agent arrive
  as events; code can answer them, or hand them to a person with `approval!`.
- Several agents run side by side as child runs, joined with `all!` or `first!`.
