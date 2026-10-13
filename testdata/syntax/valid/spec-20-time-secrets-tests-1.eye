# SPECS.md section 20, example 1.
fn averageWait(total: Duration, runs: NonZero(Int)) -> Duration = total / runs

fn percentElapsed(since: Duration, lead: Duration) -> Float = since / lead * 100.0

fn timeoutHeader(limit: Duration) -> String = "{limit.millis()}"
