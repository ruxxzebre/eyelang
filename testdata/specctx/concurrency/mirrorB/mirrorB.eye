# Another mirror of the release files, further away.

pub fn get(path: String) -> Bytes uses {wait} {
  sleep(400 ms)
  bytes(path)
}
