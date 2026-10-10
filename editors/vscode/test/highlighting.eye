# SYNTAX TEST "source.eyelang" "declarations, waiting calls, literals"

import lab.notify
# <------ keyword.control.import.eyelang
#      ^^^^^^^^^^ entity.name.namespace.eyelang

## A world's phase.
# <-- comment.line.documentation.eyelang
pub type Phase = Asleep | Running
# <--- storage.modifier.eyelang
#   ^^^^ storage.type.eyelang
#        ^^^^^ entity.name.type.eyelang
#                ^^^^^^ entity.name.type.eyelang
#                       ^ keyword.operator.variant.eyelang

entity World(id: WorldId) {
# <------ storage.type.eyelang
#      ^^^^^ entity.name.type.eyelang
  state phase: Phase = Asleep
# ^^^^^ storage.type.state.eyelang
#       ^^^^^ variable.other.property.eyelang
  migrate from 1(old) { old }
# ^^^^^^^ storage.type.migrate.eyelang
#         ^^^^ keyword.other.from.eyelang
}

@on(enters(idleWorlds))
# <- punctuation.definition.decorator.eyelang
#^^ entity.name.function.decorator.eyelang
#   ^^^^^^ entity.name.function.call.eyelang
@memory(16 MiB)
#       ^^ constant.numeric.eyelang
#          ^^^ keyword.other.unit.eyelang
job sleepWhenIdle(w: Ref(World)) -> Result((), SleepError) {
# <--- storage.type.job.eyelang
#   ^^^^^^^^^^^^^ entity.name.function.eyelang
#                                ^^ keyword.operator.arrow.eyelang
  sleep!(20 min)
# ^^^^^ keyword.control.wait.eyelang
#      ^ keyword.control.wait.eyelang
#        ^^ constant.numeric.eyelang
#           ^^^ keyword.other.unit.eyelang
  let world = w.read!()
# ^^^ storage.type.eyelang
#               ^^^^ keyword.control.wait.eyelang
  if world.players != {} { return Ok(()) }
# ^^ keyword.control.eyelang
#          ^^^^^^^ variable.other.property.eyelang
#                  ^^ keyword.operator.comparison.eyelang
#                          ^^^^^^ keyword.control.eyelang
  var n = 1_000 + 2.5e3 - 0..=9
# ^^^ storage.type.eyelang
#         ^^^^^ constant.numeric.integer.eyelang
#                 ^^^^^ constant.numeric.float.eyelang
#                          ^^^ keyword.operator.range.eyelang
  let at = 2026-10-10 07:40 in "Europe/Dublin"
#          ^^^^^^^^^^ constant.numeric.date.eyelang
#                     ^^^^^ constant.numeric.time.eyelang
#                           ^^ keyword.control.eyelang
  let s = "sleep-{w.id}-{ { a: 1 }.a }\n\q"
#         ^ punctuation.definition.string.begin.eyelang
#                ^ punctuation.section.interpolation.begin.eyelang
#                 ^ source.eyelang meta.interpolation.eyelang
#                       ^ punctuation.section.interpolation.begin.eyelang
#                         ^ punctuation.section.braces.begin.eyelang
#                                ^ - punctuation.section.interpolation.end.eyelang
#                                    ^ punctuation.section.interpolation.end.eyelang
#                                     ^^ constant.character.escape.eyelang
#                                       ^^ invalid.illegal.escape.eyelang
#                                         ^ punctuation.definition.string.end.eyelang
  let c = '\n'
#         ^^^^ string.quoted.single.char.eyelang
#          ^^ - constant.character.escape.eyelang
  let u = { user with age: 4 }
#                ^^^^ keyword.other.with.eyelang
  match x { Some(v) => v, _ => 0 }
# ^^^^^ keyword.control.eyelang
#           ^^^^ entity.name.type.eyelang
#                   ^^ keyword.operator.arrow.eyelang
#                         ^ variable.language.wildcard.eyelang
  xs |> map(fn x: x * 2)?
#    ^^ keyword.operator.pipe.eyelang
#           ^^ storage.type.function.eyelang
#              ^ - entity.name.function.eyelang
#                       ^ keyword.operator.error.eyelang
}

test "an idle world sleeps" {
# <---- storage.type.test.eyelang
#    ^ punctuation.definition.string.begin.eyelang
  with clock(2026-10-10 12:00 in "Europe/Dublin")
# ^^^^ keyword.other.with.eyelang
}

# Contextual keywords are names elsewhere.
let type = state.view + test
#   ^^^^ - storage.type.eyelang
#          ^^^^^ - storage.type.state.eyelang
#                ^^^^ variable.other.property.eyelang

fn deliver(d: Delivery) -> Option(ReceiptId) = d.receipt
# <-- storage.type.function.eyelang
#  ^^^^^^^ entity.name.function.eyelang

pub extern fn sqlite_open(path: CString) -> Int uses ffi
#   ^^^^^^ storage.modifier.extern.eyelang
#             ^^^^^^^^^^^ entity.name.function.eyelang
#                                               ^^^^ keyword.other.eyelang

contract Notify {
  command send(to: UserId) -> Delivery
# ^^^^^^^ storage.type.eyelang
#         ^^^^ entity.name.function.eyelang
  event delivered: Delivered on "notify.events"
# ^^^^^ storage.type.event.eyelang
#                            ^^ keyword.other.on.eyelang
}

let small = if n < 2 then n else fib(n - 1)
#                    ^^^^ keyword.control.eyelang
let then = 1
#   ^^^^ - keyword.control.eyelang
