# SPECS.md section 18, example 1.
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
