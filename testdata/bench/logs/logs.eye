# Text: generate a server log, then parse it line by line and sum it up per
# service: requests, mean and worst latency, and errors.

type Stats = { requests: Int, total: Int, worst: Int, errors: Int }

fn generate(n: Int) -> String {
  let services = ["auth", "billing", "search", "cart", "users", "mail", "images", "orders"]
  let levels = ["INFO", "INFO", "INFO", "WARN", "ERROR"]
  var text = ""
  var seed = 1
  for i in 0..n {
    seed = (seed * 1103515245 + 12345) % 2147483648
    let service = services[seed / 8 % 8] ?? ""
    let level = levels[seed / 64 % 5] ?? ""
    let ms = seed / 512 % 900 + 1
    text += "{i} {level} service={service} ms={ms} path=/api/{service}/{seed % 10_000}\n"
  }
  text
}

# The value of a `key=value` field.
fn value(field: String) -> String = match field.split("=") {
  [_, v] => v
  _ => ""
}

pub fn main(args: List(String)) uses {stdio} {
  let n = match parseInt(args[1] ?? "") {
    Ok(n) => n
    Err(_) => 300_000
  }
  let text = generate(n)
  var stats: Map(String, Stats) = {}
  var bad = 0
  for line in text |> lines {
    let [_, level, service, ms, ..] = line.split(" ") else {
      bad += 1
      continue
    }
    let Ok(latency) = parseInt(value(ms)) else {
      bad += 1
      continue
    }
    let name = value(service)
    let s = stats[name] ?? { requests: 0, total: 0, worst: 0, errors: 0 }
    stats = stats.insert(name, {
      requests: s.requests + 1,
      total: s.total + latency,
      worst: if latency > s.worst then latency else s.worst,
      errors: s.errors + (if level == "ERROR" then 1 else 0),
    })
  }
  for name in stats |> keys |> sort {
    let Some(s) = stats[name] else { continue }
    let mean = match nonzero(s.requests) {
      Some(r) => s.total / r
      None => 0
    }
    print("{name}: {s.requests} requests, mean {mean} ms, worst {s.worst} ms, {s.errors} errors")
  }
  print("{bad} lines not understood")
}
