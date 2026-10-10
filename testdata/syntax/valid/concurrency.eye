fn fetchAll(urls: List(Url)) -> List(Page) uses {net} =
  urls |> parallelMap(fn u: http.get(u), limit: 8)

fn dashboard(id: UserId) -> (User, List(Order)) uses {net} {
  let (user, orders) = both(fn { users.get(id) }, fn { orders.list(id) })
  (user, orders)
}

fn fastest(path: String) -> Option(Page) uses {net, clock} {
  timeout(5 s, fn {
    race([fn { mirrorA.get(path) }, fn { mirrorB.get(path) }])
  })
}

fn stream(path: String) uses {fs} {
  taskGroup(fn group {
    let (send, receive) = channel(capacity: 64)
    group.spawn(fn {
      for line in fs.lines(path) { send.put(line) }
      send.close()
    })
    for line in receive { handle(line) }
  })
}
