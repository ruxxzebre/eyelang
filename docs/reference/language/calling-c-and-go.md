---
title: "Calling C and Go"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Calling C and Go

```eye
@library("z")
extern fn crc32(crc: UInt64, data: Bytes, length: UInt32) -> UInt64

@host("lab.desk")                  # provided by the Go program that embeds Eyelang
extern fn showOnDesk(text: String) -> Result((), String)

extern type Db                     # owned by foreign code: only extern functions make one
@library("sqlite3")
extern fn open(path: String) -> Result(Db, String)
```

- An `extern fn` has a signature and no body. Calling one has the `ffi` effect, which public
  signatures state like any other, and in a job the call is a `!` step: `open!(path)`
  ([section 12](/reference/language/effects-and-signatures)).
- `@library` links a C function by its symbol. The interpreter loads it from the library at run
  time, without cgo, so `eye` itself needs no C toolchain; a compiled program calls it directly,
  linked like any other symbol. `@host` names a function that the Go program embedding Eyelang
  registers through the embedding API; the standard library's own primitives are provided the
  same way.
- Across the boundary go `Bool`, `Byte`, the fixed-width integers, `Float`, and `Bytes` and `String`
  (copied), and values of an `extern type`.
- The compiler trusts an extern's signature. It can't check that the foreign side is total or that
  it has no other effects, which is why `ffi` shows up in every signature that depends on one.
- **`extern type Db`** names a type whose values foreign code owns, and it is the one way to name
  one. Only extern functions make or take one, so Eyelang code can't build one or look inside it.
  It has no `Save`, `Show`, `Eq`, `Hash` or `Schema`, so it can't be saved, and so can't be live
  across a `!`, printed, compared or asked of a model: `d is a Db, which can't be saved, and it is
  still needed after a waiting call`. It is a type of its own, so a `Db` can't be passed where a
  `Socket` is expected. It may take type parameters, `extern type Cursor(row)`, and an
  `extern fn` may have a `where` clause, as a function does. The built-in types declared as
  `extern type` in `ir/builtin.eye` keep the traits they have.
- An `extern fn`'s parameters may be named `in` and `for`, the labels calls use for a zone and a
  duration: `daily(at: Time, in: Zone)`. A function with a body can't, since it couldn't read
  them: they are keywords.
- **eye's own built-ins are declared the same way**, in `ir/builtin.eye`: an `extern type` for each
  built-in type, such as `Int` and `List(a)`, and an `extern fn` for each built-in function, each
  with its `##` documentation. Programs never import it. The checker checks a call of a built-in
  against its declaration as against any function's, and its `uses` clause gives the call's
  effects: `now() -> Instant uses {clock}`, and `uses {wait}` for those that wait, such as `sleep`.
  Tools read it too, so hover, Go to Declaration and `eye doc` show the built-ins as they show any
  declaration. Built-ins one signature can't describe yet, such as the collection functions,
  which take any kind of collection, and `print`, which takes any number of values, keep a rule
  in the checker, and their declarations document them.
