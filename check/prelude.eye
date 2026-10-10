# The prelude: what every module sees without importing it.

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
