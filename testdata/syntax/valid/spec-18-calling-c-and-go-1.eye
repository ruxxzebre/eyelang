# SPECS.md section 18, example 1.
@library("z")
extern fn crc32(crc: UInt64, data: Bytes, length: UInt32) -> UInt64

@host("lab.desk")                  # provided by the Go program that embeds Eyelang
extern fn showOnDesk(text: String) -> Result((), String)

extern type Db                     # owned by foreign code: only extern functions make one
@library("sqlite3")
extern fn open(path: String) -> Result(Db, String)
