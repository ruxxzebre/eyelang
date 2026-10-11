# SPECS.md section 12, example 4.
import bus

job ping(day: Date) {
  bus.pinged.publish!({ n: 1 })              # a step: published when it commits
  let child = digest.start!(day)             # a step: a resumed run doesn't start it twice
  log.info("started {day}")                  # not a step
  sleep!(1 min)
}
