# `then` takes expressions, braces take blocks.
fn sign(n: Int) -> Int = if n > 0 then 1 else if n < 0 then -1 else 0

fn describe(w: World) -> String {
  let size = if w.players == {} then "empty" else "{w.players |> size} players"
  if w.phase == Asleep then log.info("asleep")
  let wait = if w.phase == Running
    then 20 min
    else 0 min
  if w.players |> size > 100 {
    log.warn("crowded")
    alert(w.id)
  } else if w.phase == Waking then log.info("waking")
  let then = "still a name"
  "{size}, {then}"
}

let pick = fn x: if x then { kind: "yes" } else { kind: "no" }
