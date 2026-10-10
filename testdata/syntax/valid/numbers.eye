fn literals() {
  let ints = [0, 42, 1_000_000, 007]
  let floats = [1.5, 0.25, 6.02e23, 1_000.000_1, 2.5E-3]
  let durations = [500 ms, 1 s, 20 min, 3 h, 1 day, 2 days, 1 week, 3 weeks, 1.5 h]
  let sizes = [512 B, 64 KiB, 64 MiB, 6 GiB]
  let dates = [2026-10-10, 2026-10-10 - 1 day]
  let times = [07:40, 23:59:30]
  let zoned = [2026-10-10 07:40 in "Europe/Dublin", 07:45 in "Europe/Dublin"]
  let subtraction = 2026 - 10 - 10
  let negative = -1
  let average = total / 3
  let remainder = total % 7
}
