---
title: "Calling C and Go"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Calling C and Go

```eye
@library("z")
extern fn crc32(crc: UInt64, data: Bytes, length: UInt32) -> UInt64

extern type GzFile                 # owned by foreign code: only extern functions make one
@library("z")
extern fn gzopen(path: String, mode: String) -> Option(GzFile)   # None for C's NULL

@host("lab.desk")                  # provided by the Go program that embeds Eyelang
extern fn showOnDesk(text: String) -> Result((), String)

extern type Db
@host("lab.sqlite")
extern fn open(path: String) -> Result(Db, String)
```

- An `extern fn` has a signature and no body. Calling one has the `ffi` effect, which public
  signatures state like any other, and in a job the call is a `!` step: `showOnDesk!(text)`
  ([section 12](/reference/language/effects-and-signatures)).
- Every `extern fn` of a program says where its code is, with exactly one of `@library` and
  `@host`: `crc has no body, so it needs @library("…"), naming the C library it is in, or
  @host("…"), naming the function the Go program that embeds Eyelang provides`.
- `@library` links a C function by its symbol, the extern fn's name. The interpreter loads it
  from the library at run time, without cgo, so `eye` itself needs no C toolchain; a compiled
  program calls it directly, linked like any other symbol. `@library("z")` is found as the system
  names libraries: `libz.so`, else the newest `libz.so.N`, on Linux, `libz.dylib` on macOS, and
  `z.dll` on Windows; a file name or a path is used as given.
- `@host` names a function that the Go program embedding Eyelang registers through the embedding
  API, the Go package `eyelang/host`; the standard library's own primitives, such as `http.get`,
  are `@host` functions too, which the interpreter provides.

  ```go
  p, err := host.Load("desk")
  err = p.Register("lab.desk", func(text string) error { return desk.Show(text) })
  code := p.Run(ctx, host.RunOptions{Stdout: os.Stdout, Stderr: os.Stderr})
  ```

- Across the boundary go these types. The checker reports any other in a `@library` function's
  signature at its span; what a `@host` function takes depends on what provides it, so `Register`
  checks it (the interpreter's own take any value):

  | Eyelang | C (`@library`) | Go (`@host`) |
  |---|---|---|
  | `Bool` | `bool` | `bool` |
  | `Byte`, `Int32`, `Int64`, `UInt32`, `UInt64` | `uint8_t`, `int32_t`, `int64_t`, `uint32_t`, `uint64_t` | `uint8`, `int32`, `int64`, `uint32`, `uint64` |
  | `Int` | none: `C has no integer of Int's unbounded size` | `int`; a value that doesn't fit is a fault |
  | `Float` | `double` | `float64` |
  | `String` | `const char *`, copied and ended with NUL; a returned one is copied, not freed | `string` |
  | `Bytes` | `const uint8_t *` to a copy, as a parameter only: pass its length too | `[]byte`, copied |
  | an `extern type` | a pointer; NULL is a fault | any pointer, struct, map, channel, function or interface type |
  | `Option` of an `extern type` | a pointer that may be NULL, which is `None` | none |
  | `()` as the result | `void` | no result |
  | `Result(T, String)` as the result | none | `(T, error)`, or `error` for `Result((), String)` |

  A Go function may also take a `context.Context` first, which is done when the task that made
  the call is cancelled.
- What can be checked is checked before anything runs. `Register` compares the Go function's
  type with each declaration of its name and returns an error naming the first difference:
  `@host("lab.add") is add(a: Int32, b: Int32) -> Int32 at desk/main.eye:8:1: parameter a is
  Int32, which Go holds as int32, and the Go function's parameter 1 is int64`. Before `main`
  starts, running a program reports each `@host` function nothing registered, and each
  `@library` function whose library or symbol can't be loaded; a test fails at such a call. A
  host function that panics, a `Db` made by one host function passed to another that takes a
  different Go type, and NULL where no `Option` allows it are faults at the call. C's parameter
  types are recorded nowhere a loader can read, so a C function declared with the wrong ones gives
  wrong values or crashes.
- Loading C libraries needs the system's dynamic linker, so on Linux the C loader makes a binary
  dynamically linked. The released `eye` for Linux stays static, so it runs on a `scratch` image,
  and loads no C library; `go build -tags clib ./cmd/eye` builds one that does, and a Go program
  that embeds Eyelang imports `eyelang/host/clib` for it. On macOS and Windows `eye` always has
  it.
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
