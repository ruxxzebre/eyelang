fn greet(name: String, count: Int) -> String {
  let plain = "hello"
  let spaced = "  two  words  "
  let escaped = "tab\there\nnewline \"quoted\" back\\slash \{not interpolated\}"
  let unicode = "naïve café \u{1F600} 日本語"
  let nested = "outer {inner("deep {name}")} done"
  let math = "{count + 1} of {count * 2}"
  let record = "{ { a: 1 }.a }"
  let empty = ""
  "{plain}, {name}! {spaced}{escaped}{unicode}{nested}{math}{record}{empty}"
}
