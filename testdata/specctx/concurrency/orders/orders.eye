# What each user ordered.

import users.{UserId}

pub type Order = { user: UserId, item: String }

pub fn list(user: UserId) -> List(Order) uses {wait} {
  sleep(300 ms)
  [{ user, item: "a book" }]
}
