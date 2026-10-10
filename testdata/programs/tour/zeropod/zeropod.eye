# zeropod saves a running world to disk and restores it. It is a Go service;
# this contract is its interface, shared by both sides.

pub type WorldId = | WorldId(String)

pub type CheckpointId = | CheckpointId(String)

pub type Refusal =
  | Busy
  | NotRunning
  | DiskFull(free: Size)

pub type CheckpointOutcome =
  | Done(CheckpointId)
  | Refused(Refusal)

pub type RestoreOutcome =
  | Restored(from: CheckpointId)
  | NothingToRestore
  | RestoreRefused(Refusal)

@port(3002)
contract Zeropod {
  @key(required)
  @reconcile(checkpointStatus)
  command checkpoint(world: WorldId) -> CheckpointOutcome

  @key(required)
  command restore(world: WorldId) -> RestoreOutcome

  ## The outcome of a checkpoint; None if none with this key arrived.
  query checkpointStatus(key: Key) -> Option(CheckpointOutcome)
}
