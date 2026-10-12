# SPECS.md section 17, example 3.
# shelves/shelves.eye
pub type Shelf = { id: String, items: List(String), version: Version }

@port(3004)
contract Shelves {
  query shelf(id: String) -> Shelf

  @fresh                                  # clear(id, basedOn)
  command clear(id: String) -> Cleared

  @destructive                            # burn(id, approval)
  command burn(id: String) -> Burned
}

# main.eye: reads again when the shelf changed in between
job tidy(id: String) {
  let shelf = shelves.shelf!(id)
  match shelves.clear!(id, basedOn: shelf.version) {
    Cleared(n) => Log(id).add!("cleared {n}")
    Stale => {
      let _ = tidy.start!(id)
    }
    Expired | Uncertain(_) => ()
  }
}

# a program that serves Shelves answers from its entity's snapshot
impl shelves.Shelves {
  fn clear(id, basedOn) {
    let rack = Rack(id).read()
    if rack.gone { return Expired }
    if rack.version != basedOn { return Stale }
    Cleared(count: Rack(id).empty())
  }

  fn shelf(id) {
    let rack = Rack(id).read()
    { id, items: rack.items, version: rack.version }
  }

  fn burn(id, approval) = Burned(count: Rack(id).burn())
}
