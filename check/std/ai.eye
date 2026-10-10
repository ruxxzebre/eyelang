## Typed calls to language models. Ask for a value of any type, and the reply
## is checked against that type and decoded into it.

# SPECS.md §19. Its primitives are provided by the host.

## Why a model call gave no value.
pub type AiError =
  ## The reply didn't match the type asked for, even after asking once more
  ## with the problems listed.
  | Malformed(reply: String, problems: List(String))
  ## The model declined to answer.
  | Declined(reason: String)
  ## The provider answered with an error.
  | ProviderError(status: Int, message: String)

## Asks a model for a value of the type the result is used as: its JSON
## Schema comes from that type, and the reply is checked and decoded into it.
## In a job, `ai.generate!(…)` is a recorded step, so a resumed run neither
## calls the model again nor pays twice.
@host("ai.generate")
pub extern fn generate(
  model: String,
  system: String = "",
  prompt: String,
) -> Result(a, AiError) uses {net}
