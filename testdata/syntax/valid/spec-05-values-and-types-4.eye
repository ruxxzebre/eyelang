# SPECS.md section 5, example 4.
fn make() -> User = { name: "Ann", age: 3 }    # a User: the return type says so
let u: User = { name: "Bo", age: 4 }
let x = { name: "Cy", age: 5 }                 # error: which record type? write `let x: User = …`
