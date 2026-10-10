# Says hello to each name it is given, or to the world.

fn greeting(name: String) -> String = "Hello, {name}!"

pub fn main(args: List(String)) uses {stdio} {
  let names = args |> drop(1)
  if names == [] {
    print(greeting("world"))
  }
  for name in names { print(greeting(name)) }
}

test "greeting names who it greets" {
  expect(greeting("Ann") == "Hello, Ann!")
}
