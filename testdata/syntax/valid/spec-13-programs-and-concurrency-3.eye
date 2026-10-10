# SPECS.md section 13, example 3.
# The example is a list of statements, wrapped in a function.
fn example() {
  taskGroup(fn group {
    let (send, receive) = channel(capacity: 64)
    group.spawn(fn {
      for line in fs.lines(path) { send.put(line) }
      send.close()
    })
    for line in receive { handle(line) }
  })
}
