# In Eye there's Expressions and Statements
# Statements -> does not return any value
# Expression -> do

# Meaning that only statements we have are 'let' and 'return'
# everything else is an Expression (even 'if')

# Any expression can be bound to a name, for example:
let x = 5;

let name = "John";

let user = {"name": "Fred", "age": 69};

# yes, 'if's are expressions, evaluated to a branch based on condition
let isAdult = if (user["age"] > 18) {
    true
} else {
    false
};

puts(user);
puts(isAdult);

# There are 'let statements', but no reassignment, only "redeclaration"
# and I'll probably throw an error later, but now it's ok
