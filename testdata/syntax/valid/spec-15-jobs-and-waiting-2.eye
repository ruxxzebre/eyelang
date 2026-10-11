# SPECS.md section 15, example 2.
job watch(t: Ref(Traveller)) {
  for _ in 0..80 {                         # about an hour of 45-second loops
    let me = t.read!()
    if me.pins == [] && me.aboard == None { return }
    sleep!(45 s)
  }
  watch.start!(t)                          # carry on in a fresh run
}
