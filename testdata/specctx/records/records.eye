# What SPECS.md §5's record examples assume (context=records): the User and
# Row types, and a list of users.

type User = { name: String, age: Int }

type Row = { name: String, age: Int }

let users: List(User) = [{ name: "Ann", age: 3 }, { name: "Bo", age: 40 }]
