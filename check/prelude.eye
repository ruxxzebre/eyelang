## What every module sees without importing it: [Option], [Result], and the
## types that runs of jobs and calls to other services give back.

## A value that may be missing: `Some(value)`, or `None`. Eyelang has no null.
##
## `?` on an Option returns `None` from a function that returns an Option,
## and `x ?? default` gives the value or the default.
pub type Option(a) = Some(a) | None

## The outcome of something that can fail: `Ok(value)` or `Err(error)`.
## Eyelang has no exceptions; failures are values of this type.
##
## `?` returns the error to the caller, wrapping it in the caller's error
## type when exactly one of its variants holds the error's type.
pub type Result(a, e) = Ok(a) | Err(e)

## An error with what was being done when it happened, for programs that only
## report errors. In a function whose error type is Problem, `?` turns any
## error with `Show` into one, and `.context("reading {path}")` adds a line.
##
## It shows as its context, outermost first, then the error:
## `reading eye.toml: not a whole number: "x"`.
pub type Problem =
  | Problem(
    ## What was being done, outermost first.
    context: List(String),
    ## The error, as it shows.
    error: String,
  )

impl Show for Problem {
  fn show(self) -> String = match self {
    Problem(context, error) => (context + [error]) |> join(": ")
  }
}

## How a run of a job ended: `Completed` with what the job returned, or
## `Failed` when it stopped first. `run.join!()` waits for it.
pub type Outcome(a) = Completed(a) | Failed(Fault)

## Why a run stopped before returning. A fault isn't an error: code can't
## return or catch one, only see it in a run's outcome.
pub type Fault =
  ## The code reached `unreachable(why)`.
  | Unreachable(String)
  ## The run passed its `@deadline`.
  | DeadlinePassed
  ## A call would have gone over the run's `@budget`.
  | OverBudget
  ## The run needed more than its `@memory`.
  | OutOfMemory
  ## A service stayed out of reach for longer than its query is retried.
  | Unavailable(String)
  ## Someone stopped the run.
  | Killed

## A person's answer to `approval!(…)`. Only `Approved` holds an `Approval`,
## which is what ending an entity or a `@destructive` command needs.
pub type Decision = Approved(Approval) | Declined | Expired

## A command whose reply was lost: the other service may or may not have
## acted, and resending could do it twice. Every command can return it
## besides the variants its contract declares; `.reconcile!()` resolves it
## when the contract names a query for it.
pub type Unconfirmed = | Uncertain(key: Key)

## `==` and `!=`. Derived for every type whose parts have it.
pub trait Eq {}

## `<`, `<=`, `>` and `>=`. Derived: numbers, text and times by value,
## variants in the order they are declared, records and lists part by part.
pub trait Ord {}

## How a value is written in `"{…}"` and by `print`. Derived; an impl's
## `show` replaces the derived form.
pub trait Show {
  fn show(self) -> String
}

## What a set's elements and a map's keys need. Derived.
pub trait Hash {}

## What can be saved: entity state, a job's arguments and what a job holds
## across a `!`. Derived; a Secret, a Handle and a function have none.
pub trait Save {}

## What a model can be asked for: `ai.generate!` derives the JSON Schema from
## the type. Derived.
pub trait Schema {}

## `a + b`, with the right-hand type rhs and the result out.
pub trait Add(rhs, out) {
  fn add(self, other: rhs) -> out
}

## `a - b`.
pub trait Sub(rhs, out) {
  fn sub(self, other: rhs) -> out
}

## `a * b`.
pub trait Mul(rhs, out) {
  fn mul(self, other: rhs) -> out
}

## `a / b`.
pub trait Div(rhs, out) {
  fn div(self, other: rhs) -> out
}

## `a % b`.
pub trait Rem(rhs, out) {
  fn rem(self, other: rhs) -> out
}

## `-a`.
pub trait Neg(out) {
  fn neg(self) -> out
}
