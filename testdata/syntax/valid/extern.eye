@library("z")
extern fn crc32(crc: UInt64, data: Bytes, length: UInt32) -> UInt64

@library("c")
@symbol("getpid")
extern fn processId() -> Int32

@host("lab.desk")
pub extern fn showOnDesk(text: String) -> Result((), String) uses {ffi}

@host("lab.camera")
extern fn snapshot(camera: Handle(Camera)) -> Bytes

pub fn checksum(data: Bytes) -> UInt64 uses {ffi} =
  crc32(0, data, UInt32.from(size(data)) ?? 0)
