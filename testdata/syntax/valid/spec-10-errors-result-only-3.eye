# SPECS.md section 10, example 3.
fn summary(latencies: List(Int), sent: Int, failed: Int) -> String {
  let mean = latencies.average() ?? 0.0
  let failedPerHundred = (failed * 100).div(sent) ?? 0
  "{mean} ms on average, {failedPerHundred} of every 100 failed"
}
