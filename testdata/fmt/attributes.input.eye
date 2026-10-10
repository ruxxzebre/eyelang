@on(enters(idle))
  @priority(background)
    @memory(16 MiB)
job sleepy(w: Ref(World)) { sleep!(20 min) }

@version(3)
entity P(id: Id) {
  @audit
    pub fn rename(to: String) { name = to }
}
