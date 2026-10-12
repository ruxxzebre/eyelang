# SPECS.md section 15, example 2.
job ask(what: String) -> Decision { approval!(what, from: operator, within: 3 days) }

job timer(d: Duration) -> Decision {
  sleep!(d)
  Expired
}

job ship(build: String) {
  match firstDone!([ask.start!("ship {build}?"), timer.start!(1 h)]) {
    Completed(Approved(ok)) => deploy!(build, ok)
    Completed(_) | Failed(_) => log.info("{build} wasn't approved within the hour")
  }
}
