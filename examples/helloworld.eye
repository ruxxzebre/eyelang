# Functions are first-class citizens in Eyelang
let hello = fn (name) {
    return "Hello, " + name + "!";
}

let greeting = hello("world"); # "Hello, world!"
puts(greeting);
