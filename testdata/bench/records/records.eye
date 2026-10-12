# Records: a particle bouncing in a box, its record replaced with `with` on
# every step.

type Particle = { x: Int, y: Int, vx: Int, vy: Int, bounces: Int }

pub fn main(args: List(String)) uses {stdio} {
  let n = match parseInt(args[1] ?? "") {
    Ok(n) => n
    Err(_) => 30_000_000
  }
  let side = 1000
  var p: Particle = { x: 0, y: 0, vx: 3, vy: 7, bounces: 0 }
  for _ in 0..n {
    p = { p with x: p.x + p.vx, y: p.y + p.vy }
    if p.x < 0 || p.x > side { p = { p with vx: -p.vx, bounces: p.bounces + 1 } }
    if p.y < 0 || p.y > side { p = { p with vy: -p.vy, bounces: p.bounces + 1 } }
  }
  print("after {n} steps: at ({p.x}, {p.y}), {p.bounces} bounces")
}
