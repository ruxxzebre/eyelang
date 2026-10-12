## Requests to any URL, for services that have no contract, such as web
## pages. Services that have one are called through it instead.

# SPECS.md §17. Its primitives are provided by the host.

## What a server answered: any status, success or not.
pub type Response = { status: Int, headers: Map(String, String), body: String }

## Why there is no answer, or, for a contract query that returns
## Result(T, HttpError), an answer whose status isn't a success.
pub type HttpError =
  ## The connection failed or broke; the reason says how, in words.
  | NoAnswer(reason: String)
  ## No answer in time.
  | TimedOut
  | BadStatus(code: Int, body: String)

## Gets a URL. Headers are sent as given; `via` is the URL of an HTTP proxy
## to go through. An answer of any status is Ok; Err means there was none.
## A request that never reached the server (the connection was refused, the
## name didn't resolve) is sent again for about a minute, as a contract's
## query is, and then is `NoAnswer`, whose reason says why, as
## "www.rte.ie refused the connection, 8 times over a minute". One that
## reached it and had no answer in time is `TimedOut` at once.
@host("http.get")
pub extern fn get(
  url: String,
  headers: Map(String, String) = {},
  via: Option(String) = None,
) -> Result(Response, HttpError) uses {net}
