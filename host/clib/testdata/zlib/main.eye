# zlib's functions, called from the C library at run time (SPECS.md §18).

@library("z")
extern fn crc32(crc: UInt64, data: Bytes, length: UInt32) -> UInt64

@library("z")
extern fn zlibVersion() -> String

## A gzip file that zlib has open.
extern type GzFile

@library("z")
extern fn gzopen(path: String, mode: String) -> Option(GzFile)

@library("z")
extern fn gzwrite(file: GzFile, data: Bytes, length: UInt32) -> Int32

@library("z")
extern fn gzclose(file: GzFile) -> Int32

## The CRC-32 of a string's bytes.
pub fn checksum(s: String) -> UInt64 uses {ffi} {
  let data = bytes(s)
  crc32(0, data, UInt32.from(size(data)) ?? 0)
}

## Writes text to a gzip file at path.
pub fn writeGz(path: String, text: String) -> Result((), String) uses {ffi} {
  let Some(file) = gzopen(path, "wb") else {
    return Err("can't open {path}")
  }
  let data = bytes(text)
  let written = gzwrite(file, data, UInt32.from(size(data)) ?? 0)
  if gzclose(file) != 0 || written.toInt() != size(data) {
    return Err("wrote {written} of {size(data)} bytes to {path}")
  }
  Ok(())
}

pub fn main(args: List(String)) -> Result((), String) uses {ffi, stdio} {
  print("crc32 {checksum("hello")}, zlib {zlibVersion()}")
  let Some(path) = args[1] else {
    return Err("usage: zlib PATH")
  }
  writeGz(path, "hello, gzip")?
  writeGz("/no/such/dir/x.gz", "")
}

test "crc32 from zlib" {
  expect(checksum("hello") == 907060870)
  expect(checksum("") == 0)
}
