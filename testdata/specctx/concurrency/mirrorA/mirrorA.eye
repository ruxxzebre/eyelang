# One mirror of the release files.

pub fn get(path: String) -> Bytes uses {wait} {
  sleep(200 ms)
  bytes(path)
}
