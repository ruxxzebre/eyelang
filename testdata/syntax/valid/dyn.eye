trait Handler {
  fn handle(self, event: Event) -> Result((), Problem)
}

fn open(config: Config) -> dyn Store =
  if config.postgres { PgStore.connect(config.url) } else { MemoryStore.new() }

fn dispatch(handlers: List(dyn Handler), event: Event) -> Result((), Problem) {
  for h in handlers {
    h.handle(event)?
  }
  Ok(())
}
