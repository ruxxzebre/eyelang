## Random numbers and choices.
##
## In a test they come from the same seed every time, so a test that uses
## them gives the same result on every run. In a job, each draw is a step,
## `random.int!(1..=6)`, so a resumed run sees the numbers it drew before.

# SPECS.md §12. Its primitives are provided by the host.

## A whole number in the range, each as likely: `random.int(1..=6)`. `None`
## when the range is empty.
@host("random.int")
pub extern fn int(range: Range(Int)) -> Option(Int) uses {random}

## A number from 0 up to, but not including, 1.
@host("random.float")
pub extern fn float() -> Float uses {random}

## One of the elements, each as likely, or `None` if there are none.
@host("random.pick")
pub extern fn pick(xs: List(a)) -> Option(a) uses {random}

## The elements in a random order.
@host("random.shuffle")
pub extern fn shuffle(xs: List(a)) -> List(a) uses {random}
