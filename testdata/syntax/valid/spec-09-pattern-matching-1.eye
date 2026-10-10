# SPECS.md section 9, example 1.
# The example is a list of statements, wrapped in a function.
fn example() {
  match grant {
    Admitted(revision) => use(revision)
    Pending(receipt) if receipt.age > 1 h => escalate(receipt)
    Pending(receipt) => wait(receipt)
    Refused(..) => report(grant)
  }

  let { name, age } = user                     # destructuring
  let [first, ..rest] = items else { return Err(Empty) }
  let (key, value) = entry

  match rest {                                 # rest: Bytes
    ['-', '>', ..] => Arrow
    ['0'..='9', ..] => number(rest)
    [] => End
    _ => Illegal
  }
}
