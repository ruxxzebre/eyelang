# Floats: which points of an n×n grid over [-2, 1]×[-1.5, 1.5] stay in the
# Mandelbrot set for 100 iterations, and how many iterations it took in all.

fn escape(cr: Float, ci: Float, limit: Int) -> Int {
  var zr = 0.0
  var zi = 0.0
  var i = 0
  while i < limit && zr * zr + zi * zi <= 4.0 {
    let t = zr * zr - zi * zi + cr
    zi = 2.0 * zr * zi + ci
    zr = t
    i += 1
  }
  i
}

pub fn main(args: List(String)) uses {stdio} {
  let n = match parseInt(args[1] ?? "") {
    Ok(n) => n
    Err(_) => 1600
  }
  let limit = 100
  let size = toFloat(n)
  var inside = 0
  var steps = 0
  for y in 0..n {
    let ci = toFloat(y) * 3.0 / size - 1.5
    for x in 0..n {
      let cr = toFloat(x) * 3.0 / size - 2.0
      let i = escape(cr, ci, limit)
      steps += i
      if i == limit { inside += 1 }
    }
  }
  print("{n}x{n}: {inside} inside, {steps} iterations")
}
