# Access log summary. Reads a web server's access log and prints a table:
# for each path, how many requests it had, how many failed, and how slow it
# was. Plain Eyelang, with no jobs or entities: records and variants, errors
# as values, a trait, a generic function and pipes.

import fs

# region parse
## One request, as one line of the log records it.
pub type Request = { method: String, path: String, status: Int, ms: Int }

## Why a line couldn't be read.
pub type LineError =
  | WrongFields(found: Int)
  | NotANumber(field: String, text: String)

## How a LineError is written in a message, in place of the derived
## `NotANumber(field: "ms", text: "fast")`.
impl Show for LineError {
  fn show(self) = match self {
    WrongFields(found) => "expected 5 fields, found {found}"
    NotANumber(field, text) => "the {field} should be a number, and is \"{text}\""
  }
}

## Reads a number, or says which field held something else.
fn number(field: String, text: String) -> Result(Int, LineError) =
  parseInt(text).mapErr(fn _: NotANumber(field:, text:))

## Reads one line: `2026-10-12T09:00:01Z GET /api/orders 200 34`, where the
## last two fields are the status and the time taken, in milliseconds.
pub fn parseLine(line: String) -> Result(Request, LineError) {
  let fields = line |> words()
  let [_, method, path, status, ms] = fields else { return Err(WrongFields(found: fields |> size)) }
  Ok({ method, path, status: number("status", status)?, ms: number("ms", ms)? })
}
# endregion parse

# region stats
## The value below which `p` percent of `xs` fall, for any type that can be
## ordered: `percentile([3, 1, 2], 50)` is `Some(2)`.
pub fn percentile(xs: List(a), p: Int) -> Option(a) where a: Ord {
  let sorted = xs |> sort()
  sorted[(sorted |> size) * p / 100]
}

## What one path's requests add up to. Records are ordered by their fields
## in the order written here, so sorting these puts the least busy first.
pub type PathStats = { requests: Int, path: String, failed: Int, p50: Int, p95: Int }

## Sums up one path's requests. A 5xx status counts as failed.
fn stats(path: String, rs: List(Request)) -> PathStats {
  let times = rs |> map(fn r: r.ms)
  {
    path,
    requests: rs |> size,
    failed: rs |> count(fn r: r.status >= 500),
    p50: percentile(times, 50) ?? 0,
    p95: percentile(times, 95) ?? 0,
  }
}

## Groups requests by path, then sums up each group, busiest path first.
pub fn byPath(requests: List(Request)) -> List(PathStats) {
  var groups: Map(String, List(Request)) = {}
  for r in requests {
    groups[r.path] = (groups[r.path] ?? []) + [r]
  }
  groups
    |> entries()
    |> map(fn (path, rs): stats(path, rs))
    |> sort()
    |> reverse()
}
# endregion stats

# region table
## Anything a report shows as one row of a table.
trait Row {
  fn cells(self) -> List(String)
}

impl Row for PathStats {
  fn cells(self) = [self.path, "{self.requests}", "{self.failed}", "{self.p50} ms", "{self.p95} ms"]
}

## Pads `s` with spaces to `width` characters.
fn pad(s: String, width: Int) -> String {
  let n = s.chars() |> toList() |> size
  s + ((0..max(width - n, 0)) |> map(fn _: " ") |> join(""))
}

## Lays out rows of any type that is a Row, under a header, in columns.
pub fn table(header: List(String), rows: List(r)) -> String where r: Row {
  let lines = [header] + (rows |> map(fn r: r.cells()))
  let widths = header
    |> enumerate()
    |> map(fn (i, _): lines |> map(fn l: (l[i] ?? "").chars() |> toList() |> size) |> reduce(0, max))
  lines
    |> map(fn l: l |> zip(widths) |> map(fn (cell, w): pad(cell, w)) |> join("  ") |> trim())
    |> join("\n")
}
# endregion table

# region main
## A few lines of a log, summed up when no file is named.
let sample = """
  2026-10-12T09:00:01Z GET /api/orders 200 34
  2026-10-12T09:00:01Z GET /health 200 2
  2026-10-12T09:00:02Z POST /api/orders 201 120
  2026-10-12T09:00:03Z GET /api/orders 200 41
  2026-10-12T09:00:04Z POST /api/orders 503 2004
  2026-10-12T09:00:06Z GET /health 200 3
  2026-10-12T09:00:07Z GET /api/orders/1183 404 9
  2026-10-12T09:00:08Z the load balancer restarted
  2026-10-12T09:00:10Z GET /api/orders 500 15
  """

## Prints the summary of the log file named, or of the sample:
## `eye run . -- /var/log/nginx/access.log`.
pub fn main(args: List(String)) -> Result((), Problem) uses {fs, stdio} {
  let path = args[1] ?? "sample"
  let text = if path == "sample" then sample else fs.readText(path).context("reading {path}")?
  var requests = []
  for (n, line) in text |> lines() |> enumerate() {
    match parseLine(line) {
      Ok(r) => requests += [r]
      Err(e) => log.warn("{path}:{n + 1}: {e}")
    }
  }
  print(table(["path", "requests", "failed", "p50", "p95"], byPath(requests)))
  Ok(())
}
# endregion main

# ---------------------------------------------------------------- tests

# region tests
test "a line is read into a request, or says what is wrong with it" {
  expect(parseLine("2026-10-12T09:00:01Z GET /api/orders 200 34") == Ok({ method: "GET", path: "/api/orders", status: 200, ms: 34 }))
  expect(parseLine("GET /api/orders 200") == Err(WrongFields(found: 3)))
  expect(parseLine("2026-10-12T09:00:01Z GET / 200 fast") == Err(NotANumber(field: "ms", text: "fast")))
}

test "percentile works on anything that can be ordered" {
  expect(percentile([30, 10, 20, 40], 50) == Some(30))
  expect(percentile(["b", "a", "c"], 0) == Some("a"))
  expect(percentile([], 95) == None)
}

test "requests are summed up per path, busiest first" {
  let log: List(Request) = [
    { method: "GET", path: "/health", status: 200, ms: 5 },
    { method: "GET", path: "/api/orders", status: 200, ms: 30 },
    { method: "POST", path: "/api/orders", status: 503, ms: 900 },
  ]
  let stats = byPath(log)
  expect(stats |> map(fn s: s.path) == ["/api/orders", "/health"])
  expect(table(["path", "requests", "failed"], stats |> take(1)) == "path         requests  failed\n/api/orders  2         1")
}
# endregion tests
