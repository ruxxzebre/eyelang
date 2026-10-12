# Allocation: the benchmarks game's binary trees. Many short-lived trees of
# depth 4 to n are built and walked while one long-lived tree stays.

type Tree =
  | Leaf
  | Node(left: Tree, right: Tree)

fn make(depth: Int) -> Tree =
  if depth == 0 then Node(left: Leaf, right: Leaf) else Node(left: make(depth - 1), right: make(depth - 1))

fn check(t: Tree) -> Int = match t {
  Leaf => 0
  Node(left, right) => 1 + check(left) + check(right)
}

pub fn main(args: List(String)) uses {stdio} {
  let n = match parseInt(args[1] ?? "") {
    Ok(n) => n
    Err(_) => 16
  }
  let minDepth = 4
  let maxDepth = if n < minDepth + 2 then minDepth + 2 else n
  let stretch = maxDepth + 1
  print("stretch tree of depth {stretch}\t check: {check(make(stretch))}")
  let longLived = make(maxDepth)
  var depth = minDepth
  while depth <= maxDepth {
    var iterations = 1
    for _ in 0..(maxDepth - depth + minDepth) {
      iterations *= 2
    }
    var sum = 0
    for _ in 0..iterations {
      sum += check(make(depth))
    }
    print("{iterations}\t trees of depth {depth}\t check: {sum}")
    depth += 2
  }
  print("long lived tree of depth {maxDepth}\t check: {check(longLived)}")
}
