## What Counter tells the others.

## A count, as Counter has it after an add.
pub type Counted = { id: String, count: Int }

contract Bus {
  event counted: Counted on "bus.counted"
}
