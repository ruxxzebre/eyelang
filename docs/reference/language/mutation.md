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

- **What can change:** a `var`, a `mut` parameter, and fields of either (`c.last = …`). `+=`, `-=`,
  `*=`, `/=` and `%=` combine an operation with an assignment, so `found += [text]` appends to a
  list and `out += "\n"` to a string, in place when nothing else refers to them. A `let` never
  changes.
- **Visible at the call.** A `mut` argument is written `mut x` (or `mut x.field`) and must be a
  `var` or a `mut` parameter. A `mut` parameter is always passed that way, and only a `mut`
  parameter is. A method that takes `mut self` can only be called on a `var` or a `mut` parameter.
- **No aliasing.** The same variable can't be passed as `mut` twice in one call, or passed as `mut`
  and read in the same call: `swap(mut a, mut a)` is a compile error. Closures can't capture a
  `var` or a `mut` parameter. An entity method's closures may read its state fields, though
  ([section 14](/reference/language/entities)).
- **No references.** A `mut` parameter lasts for the call; it can't be stored or returned. Nothing
  in the language is a pointer, so values can't form cycles, and reference counting reclaims
  everything without a garbage collector.
- **In place when unique.** A change to a value that nothing else refers to happens in place. If
  something does, the value is copied first (copy on write), so the change stays local.
- **Entity methods** are `mut self` methods over the entity's state, run as a transaction
  ([section 14](/reference/language/entities)).
