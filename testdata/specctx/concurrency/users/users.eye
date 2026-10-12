# Who the users are.

pub type UserId = | UserId(String)

pub type User = { id: UserId, name: String }

pub fn get(id: UserId) -> User uses {wait} {
  sleep(300 ms)
  { id, name: "Ann" }
}
