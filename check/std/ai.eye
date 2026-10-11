## Typed calls to language models. Ask for a value of any type, and the reply
## is checked against that type and decoded into it.

# SPECS.md §19. Its primitives are provided by the host.

# == Model calls

## Why a model call gave no value.
pub type AiError =
  ## The reply didn't match the type asked for, even after asking once more
  ## with the problems listed.
  | Malformed(reply: String, problems: List(String))
  ## The model declined to answer.
  | Declined(reason: String)
  ## The provider answered with an error.
  | ProviderError(status: Int, message: String)

## Who answers a model call.
pub type Provider =
  ## Anthropic's Messages API, at `ANTHROPIC_URL` or api.anthropic.com, with
  ## the key `secret("anthropic-api-key")`.
  | Anthropic
  ## OpenAI's chat completions, at `OPENAI_URL` or api.openai.com, with the
  ## key `secret("openai-api-key")`.
  | OpenAI
  ## An OpenAI-compatible server at `AI_URL`, such as a gateway that serves
  ## many models, with the key `secret("ai-key")` if it needs one.
  | Gateway

## Asks a model for a value of the type the result is used as: its JSON
## Schema comes from that type, and the reply is checked and decoded into it.
## In a job, `ai.generate!(…)` is a recorded step, so a resumed run neither
## calls the model again nor pays twice.
##
## Without `provider:`, a gateway answers every call when `AI_URL` is set;
## otherwise the model's name decides: `claude-…` is [Anthropic], `gpt-…` and
## `o1`, `o3`, `o4` … are [OpenAI].
##
## A `@budget` on the job or function counts each reply's tokens and price. A
## call is sent only if the budget leaves room for its prompt and some reply,
## and the reply is cut off where the budget ends; a call that doesn't fit
## ends the run as `Failed(OverBudget)`.
@host("ai.generate")
pub extern fn generate(
  model: String,
  system: String = "",
  prompt: String,
  provider: Option(Provider) = None,
) -> Result(a, AiError) uses {net}
