# SPECS.md section 8, example 2.
type Span = { from: Int, to: Int }

fn moved(s: Span, by: Int) -> Span = { from: s.from + by, to: s.to + by }
fn twice(n) = n + n

fn shifted(spans: List(Span)) -> List(Span) =
  spans.map(fn s: s.moved(by: 2.twice()))      # moved(s, by: twice(2))
