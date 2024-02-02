# Functions are first-class citizens in Eyelang
# and are expressions that can be assigned to variables

let vectorLength = fn (x, y, z) {
    return math.sqrt(math.pow(x, 2) + math.pow(y, 2) + math.pow(z, 2))
}

puts(vectorLength(3, 3, 3))

# plain expression which WILL (believe me) get GC'd
fn (name) {
    return "Hello, " + name + "!"
}

# In fact, 'return' is optional
let isPositive = fn (num) {
    if (num > 0) {
        "yes"
    } else {
        "no"
    }
}

# Currying is also an option:

# Function that checks if `num` is in range between low and high
let clampTime = fn (lo, hi, num) {
    if (num < lo) { return lo } # yes, returns are optional, but not here,
    # as we'll just fallthrough to the next if

    if (num > hi) { return hi }

    return num
}

# providing only 2 args
let clampNight = clampTime(0, 9)

puts(clampNight)
puts(clampNight(3)) # -> gives 3
puts(clampNight(11)) # -> gives 9
