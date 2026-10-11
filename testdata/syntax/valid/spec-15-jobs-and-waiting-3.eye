# SPECS.md section 15, example 3.
import approvals

@on(event(approvals.requested))
job ask(r: approvals.Request) {
  let left = r.until - now!()
  let asked = notify.ask!(to: operator, text: r.what, choices: [yes, no], within: left,
    key: "approval-{r.id}").reconcile!()
  let Asking(question) = asked else { return }
  let Some(press) = next!(notify.answered, where: fn a: a.question == question, within: left) else {
    return
  }
  approvals.answered.publish!({ id: r.id, approved: press.choice == yes.id, by: "{press.by}" })
}
