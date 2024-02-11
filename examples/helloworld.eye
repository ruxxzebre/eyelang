load("./examples/iters.eye");

# Functions are first-class citizens in Eyelang
let hello = fn (name) {
    return "Hello, " + name + "!";
}

let greeting = hello("world"); # "Hello, world!"
puts(greeting);

let newUser = fn (name, age) {
    {
        "id": math.random(),
        "name": name,
        "age": age
    }
}

let alan = newUser("alan", 20);

let users = map([1, 2, 3, 4, 5], fn (x) {
    return newUser("user", x);
});

puts(users);
