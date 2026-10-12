@on(cron("*/5 * * * *", in: "UTC"))
job poll() {
  let run = report.start(today(), after: 20 min)
  let other = report.start(today(), at: 07:40 in "Europe/Dublin")
  allDone!([run, other])
  let winner = firstDone!([sleeper(), approval!("ship it?", from: operator, within: 3 days)])
  lock!("ci", for: 30 min, fn {
    build!()
  })
}

job report(day: Date) -> Summary { summarise!(day) }
