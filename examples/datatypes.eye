# Built-in objects in Eye:

# INTEGER

let x = 5;
puts(x * x);
x / 9;
x + 5;

# BOOLEAN

let truthy = true;
let falsy = false;

let moreThanThree = x > 3;
if (moreThanThree) {
    puts("x is more than 3! got=" + x);
}

# FUNCTION

puts("Hello World!"); # builtin function which prints expression to the screen

# as functions are not statements, they should be bound to variables this way:
let isLongPassword = fn (password) {
    return len(password) > 9;
}

isLongPassword("smolboi2002");

# STRING

let greeting = "hello";

# ARRAY

let bets = [6, 10, 2, 5];

# any expression can be an item of the array
let conditions = [x > 5, moreThanThree, bets];

# HASH

let user = {
    "name": "John",
    "age": 6
};

# any HASHABLE expression can be a KEY:
let rbg = {
    true: 100,
    false: 255
};

# yes you can do that
puts(rbg[true]);

# There are special types, which cannot be created by user (for now) like those:
# BUILTIN - builtin function type, implemented by host language, like `puts`, `len`, and methods on builtin modules
# ERROR - error expression, returned when evaluator encounters type error
# SCOPE - returned by `load` function
# MACRO - `load` function for now
# NULL - returned by functions which don't return error (yet)

# RETURN_VALUE
# ahh?
