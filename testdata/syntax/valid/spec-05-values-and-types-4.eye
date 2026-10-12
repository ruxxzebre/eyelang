# SPECS.md section 5, example 4.
let rows: List(Row) = users |> map(fn u: { name: u.name, age: u.age })   # each a Row

fn adults(users: List(User)) -> List(Row) =
  users |> map(fn u: { name: u.name, age: u.age }) |> filter(fn r: r.age >= 18)   # r is a Row

let names = users |> map(fn u: { name: u.name }) |> map(fn r: r.name)   # error: which record type?
