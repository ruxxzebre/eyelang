# The typed IR of entities and jobs: method targets, durable steps and what
# waits (eye check --ir).

type Phase = Idle | Brewing | Done

entity Kettle(id: String) {
  state phase: Phase = Idle

  pub fn brew() {
    if phase == Idle {
      phase = Brewing
    }
  }

  pub fn finish() { phase = Done }
}

job boil(id: String) {
  Kettle(id).brew!()
  sleep!(3 min)
  let k = Kettle(id).read!()
  match k.phase {
    Brewing => Kettle(id).finish!()
    Idle | Done => ()
  }
}
