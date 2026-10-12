## How urgent a message is, and a sender's state.

## How urgent a message is.
pub type Priority = Low | Normal | High

## A sender's state: busy for some seconds, or idle.
pub type Status =
  | Busy(secs: Int)
  | Idle

## The priority a message is sent with.
pub fn send(priority: Priority) -> Priority = priority
