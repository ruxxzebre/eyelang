# Irish Rail's real-time feed, for the trains a person has pinned.

pub type TrainCode = | TrainCode(String)

pub type StopId = | StopId(String)

## Where a train is expected to be, as the feed last said.
pub type Movement = {
  train: TrainCode,
  date: Date,
  departures: Map(StopId, Instant),
  arrivals: Map(StopId, Instant),
  ## How late the train runs now; zero when on time.
  delay: Duration,
  cancelled: Bool,
}

contract IrishRail {
  ## None when the feed doesn't know the train on that date.
  query movement(train: TrainCode, date: Date) -> Option(Movement)
}
