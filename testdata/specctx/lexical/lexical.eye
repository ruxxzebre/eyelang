# What SPECS.md §4's examples assume (context=lexical): a reader to write
# to, a list of worlds, and two values.

type Phase = Asleep | Running

type World = { name: String, phase: Phase }

let reader = "Paul"

let worlds: List(World) = [{ name: "survival", phase: Running }]

let a = 1

let b = 2
