# Recursion too deep in a compiled job: a fault that fails this test, in the
# runner as in the interpreter, and never a crash. TestHostedFault runs it.

fn depth(n: Int) -> Int = if n == 0 then 0 else 1 + depth(n - 1)

job tooDeep() -> Int {
  sleep!(1 min)
  depth(100000000)
}

test "recursion too deep" {
  let _ = tooDeep.start(key: "d")
  advance(10 min)
}

job failing() {
  sleep!(1 min)
  unreachable("gave up here")
}

test "a fault in a job" {
  let _ = failing.start(key: "f")
  advance(10 min)
}
