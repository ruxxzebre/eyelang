# SPECS.md section 8, example 2.
fn notifier(config: Config) -> dyn Notifier =
  if config.telegram then Telegram.new(config.token) else Console.new()

let sinks: List(dyn Sink) = [Logger.new(), Metrics.new(port: 9100)]
