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
```

- An `extern fn` has a signature and no body. Calling one has the `ffi` effect, which public
  signatures state like any other.
- `@library` links a C function by its symbol. `@host` names a function that the Go program
  embedding Eyelang registers through the embedding API; the standard library's own primitives are
  provided the same way.
- Across the boundary go `Bool`, `Byte`, the fixed-width integers, `Float`, and `Bytes` and `String`
  (copied). An object owned by foreign code is a `Handle(name)`, which can't be saved, so it can't
  be live across a `!`.
- The compiler trusts an extern's signature. It can't check that the foreign side is total or that
  it has no other effects, which is why `ffi` shows up in every signature that depends on one.
