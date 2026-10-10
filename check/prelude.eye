# The prelude: what every module sees without importing it.

pub type Option(a) = Some(a) | None

pub type Result(a, e) = Ok(a) | Err(e)
