---
title: "Mutation"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Mutation

Eyelang has mutable value semantics, as Swift's value types and Hylo do. A value never changes
behind your back: the only things that change are `var` variables and `mut` parameters, and a
change is visible only through the variable it was made to.

```eye
type Counter = { hits: Int, last: Option(Instant) }

fn record(mut c: Counter, at: Instant) {      # may change the caller's counter
  c.hits += 1
  c.last = Some(at)
}

impl Counter {
  fn reset(mut self) { self.hits = 0 }
}

fn tally(times: List(Instant)) -> Counter {
  var counter = { hits: 0, last: None }
  for t in times {
    record(mut counter, t)                    # `mut` at the call: this call may change counter
  }
  let snapshot = counter                      # a copy; later changes to counter don't reach it
  counter.reset()                             # a `mut self` method needs a `var` receiver
  snapshot
}
```

- **What can change:** a `var`, a `mut` parameter, and fields and elements of either
  (`c.last = …`, `xs[i] = …`). `+=`, `-=`,
  `*=`, `/=` and `%=` combine an operation with an assignment, so `found += [text]` appends to a
  list and `out += "\n"` to a string, in place when nothing else refers to them. A `let` never
  changes.
- **Elements.** `xs[i] = v` changes one element of a `List`, and `xs[i] += v` and the other
  compound forms combine it with an operation, as for fields; steps chain, so
  `points[i].x = 0`, `grid[y][x] = '#'` and `self.open[n - 1] = s` all work. The change is in
  place when nothing else refers to the list, and a copy's otherwise, so a name that saw the list
  before still sees it as it was. The value is computed first, then the indexes, left to right.

  ```eye
  fn histogram(data: Bytes) -> List(Int) {
    var counts = (0..256) |> map(fn _: 0)
    for b in data {
      counts[b.toInt()] += 1
    }
    counts
  }
  ```

  - **An index out of range is a fault**, `index 3 is out of range for a list of 3`, which ends
    the run as `unreachable` does ([section 10](/reference/language/errors-result-only)). Reading an element gives an `Option`, since a
    missing one is a case code handles; writing where there is no element is a bug, and doing
    nothing would lose the write without a word. A list grows with `xs += [v]`.
  - **`m[key] = v`** inserts the key into a `Map`, or replaces its value, and is total. It is the
    one form for maps: the key may be missing, so `m[key] += v` and `m[key].field = v` are errors,
    and the first says to write `m[key] = (m[key] ?? 0) + v`.
  - `Bytes` don't change one byte at a time, since their slices share storage ([section 5](/reference/language/values-and-types)): build
    a `List(Byte)` and make `Bytes` with `toBytes`. A slice, `xs[a..b]`, can't be assigned, and an
    element can't be passed as `mut`: copy it into a `var`, pass that, and assign it back.
- **Visible at the call.** A `mut` argument is written `mut x` (or `mut x.field`) and must be a
  `var` or a `mut` parameter. A `mut` parameter is always passed that way, and only a `mut`
  parameter is. A method that takes `mut self` can only be called on a `var` or a `mut` parameter.
- **No aliasing.** The same variable can't be passed as `mut` twice in one call, or passed as `mut`
  and read in the same call: `swap(mut a, mut a)` is a compile error. A lambda given to the call
  that reads the variable reads it in the call too.
- **Only callbacks read what can change.** A lambda copies what it captures when it is made. A
  *callback*, a lambda given straight to a parameter whose function type has no `uses`
  ([section 12](/reference/language/effects-and-signatures)), runs before the call returns, so it may read a `var`, a `mut` parameter, and
  `self` in a `mut self` method, an entity's state included: it reads their values at the call.
  It can't assign them; it returns what it computes, and the code around it assigns. Any other
  lambda, one kept in a `let`, a field or a collection, or returned, could run after they
  change, so it can't capture them: copy what it needs into a `let` first.

  ```eye
  type Post = { links: List(String) }

  fn links(feed: List(Post)) -> Set(String) {
    var seen: Set(String) = {}
    for post in feed {
      let fresh = post.links |> filter(fn l: !(seen |> contains(l)))   # a callback reads seen
      seen = seen + toSet(fresh)
    }
    let isNew = fn l: !(seen |> contains(l))    # error: a closure can't capture seen, a var
    seen
  }
  ```
- **No references.** A `mut` parameter lasts for the call; it can't be stored or returned. Nothing
  in the language is a pointer, so values can't form cycles, and reference counting reclaims
  everything without a garbage collector.
- **In place when unique.** A change to a value that nothing else refers to happens in place. If
  something does, the value is copied first (copy on write), so the change stays local.
- **Entity methods** are `mut self` methods over the entity's state, run as a transaction
  ([section 14](/reference/language/entities)).
