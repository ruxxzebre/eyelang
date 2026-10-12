# SPECS.md section 15, example 2.
fn retry(times: Int, call: fn() -> Bool) -> Bool uses {wait} {
  var tries = 0
  var ok = call()
  while !ok && tries < times {
    sleep!(1 min)
    tries += 1
    ok = call()
  }
  ok
}
