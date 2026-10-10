# SPECS.md section 8, example 2.
fn open(config: Config) -> dyn Store =
  if config.postgres { PgStore.connect(config.url) } else { MemoryStore.new() }

let handlers: List(dyn Handler) = [Logger.new(), Metrics.new(port: 9100)]
