type User = { name: String, age: Int, tags: Set(String) }

fn records(user: User, key: String) {
  let ann = { name: "Ann", age: 3, tags: {} }
  let older = { ann with age: 4 }
  let both = { older with name: "Annie", tags: toSet(["a", "b"]) }
  let name = "Bo"
  let age = 7
  let punned = { name, age, tags: {} }
  let scores = { "alice": 3, "bob": 5 }
  let byDay = { 2026-10-10: "sat", 2026-10-11: "sun" }
  let byTime = { 07:40: "breakfast" }
  let computed = { (key): 1, (key + "2"): 2 }
  let numbered = { 1: "one", 2: "two" }
  let empty = {}
  let updated = { user with tags: user.tags |> add("new") }
}
