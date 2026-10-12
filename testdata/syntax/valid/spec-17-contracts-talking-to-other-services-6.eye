# SPECS.md section 17, example 6.
test "the morning's briefing goes at 07:40, for the Irish day" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  with clock(2026-10-12 06:30 in "UTC")
  advance(10 min)
  let [briefing] = sent.calls(notify.send) else { unreachable("sent more or less than once") }
  expect(briefing.key == "ireland-2026-10-12" && briefing.priority == Normal)
}
