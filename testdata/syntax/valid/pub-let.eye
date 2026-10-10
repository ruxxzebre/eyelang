# `pub let` exports a module-level value; other modules import it by name.

pub let operator = UserId("operator")

## The lab runs on Irish time.
pub let dublin: Zone = "Europe/Dublin"

let local = 3

pub let limits: List(Int) = [1, 2, local]
