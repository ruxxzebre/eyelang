# SPECS.md section 13, example 3.
# The example is a list of statements, wrapped in a function.
fn example() {
  taskGroup(fn group {
    let (send, receive) = channel(capacity: 64)
    group.spawn(fn {
      for url in feeds { send.put(fetch(url)) }
      send.close()
    })
    for page in receive { handle(page) }
  })
}
