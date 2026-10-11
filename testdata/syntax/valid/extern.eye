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

# A type foreign code owns, with and without type parameters.
extern type Db
pub extern type Cursor(row)

@library("sqlite3")
extern fn open(path: String) -> Result(Db, String) uses {ffi}

@host("lab.sort")
extern fn sorted(rows: List(row)) -> List(row) where row: Ord
