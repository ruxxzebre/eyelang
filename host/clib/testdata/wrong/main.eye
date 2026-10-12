# C functions that can't be called as declared (clib_test.go).

@library("eyelang-no-such-library")
extern fn nothing() -> Int32

@library("z")
extern fn noSuchZlibFunction() -> Int32

extern type GzFile

## gzopen gives NULL when it can't open the file, which this declaration
## doesn't allow for: its result should be Option(GzFile).
@library("z")
extern fn gzopen(path: String, mode: String) -> GzFile

test "a library that isn't there" {
  expect(nothing() == 0)
}

test "a symbol the library doesn't have" {
  expect(noSuchZlibFunction() == 0)
}

test "NULL where the declaration has no Option" {
  let _ = gzopen("/no/such/dir/x.gz", "rb")
}
