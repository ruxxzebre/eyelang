# What the lab's programs share: people, and the time zone the lab lives in.

## Someone with an account in the lab's identity provider (Authentik).
pub type UserId = | UserId(String)

## The lab runs on Irish time: schedules, quiet hours and service dates.
pub let dublin: Zone = "Europe/Dublin"

## Who is asked when something needs a person.
pub let operator = UserId("operator")
