# A store's shelves: a @fresh command that can't act on a shelf changed since it was read, and a
# @destructive one that needs a person's approval.

pub type Shelf = { id: String, items: List(String), version: Version }

pub type Cleared = | Cleared(count: Int)

pub type Burned = | Burned(count: Int)

@port(3004)
contract Shelves {
  query shelf(id: String) -> Shelf

  ## Empties a shelf, unless it changed after `basedOn`.
  @fresh
  command clear(id: String) -> Cleared

  ## Burns a shelf down, with a person's approval.
  @destructive
  command burn(id: String) -> Burned
}
