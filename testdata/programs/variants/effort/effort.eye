## How hard a model thinks, and its reply.

## How hard to think.
pub type Effort = Low | Medium | High

## A reply: busy for some seconds, or fine.
pub type Reply =
  | Busy(secs: Int)
  | Fine

## The effort asked for, or Medium.
pub fn think(effort: Option(Effort)) -> Effort = effort ?? Medium
