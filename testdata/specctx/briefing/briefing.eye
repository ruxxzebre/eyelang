# What SPECS.md §17's test of a fake assumes (context=notify,briefing): the
# job that sends the morning's briefing through Notify, and a recording of
# Notify's answer (recordings/notify.json).

import notify.{UserId}

let owner = UserId("operator")

@on(daily(07:40, in: "Europe/Dublin"))
job briefing(day: Date) -> notify.Delivery {
  notify.send!(owner, "Good morning: the news for {day}", key: "ireland-{day}").reconcile!()
}
