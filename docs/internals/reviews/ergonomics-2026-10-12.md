# How nice Eyelang is to type, 2026-10-12

An analysis of what it is like to write, read and change ordinary Eyelang programs, by hand and
with an editor, as a person and as a coding agent. It was written by a reviewing agent on
2026-10-12 against `dev` at `6f04ce9`, with `eye` built from that commit.

The method: seven small but realistic programs, written from the spec and the guides the way a
newcomer would, each taken from a first attempt to `eye check` and `eye test` passing, with every
stumble noted. Five of them were also written in Python, Go, TypeScript or Rust, and the Eyelang
door alarm against Temporal's Python SDK, all run to green. Around 60 deliberate mistakes, the
kind a person or an agent coming from another language makes, were then fed to `eye check` to
collect its messages. The language server, `eye fmt`, `eye doc`, `eye test`, `eye runs` and
`eye repl` were driven from the command line. The scratch programs are not in the repository;
the snippets below are taken from them.

The [spec review](./spec-review-2026-10-12.md) and the
[performance analysis](./performance-and-packages-2026-10-12.md) of the same day are context.
This page doesn't repeat them: it is about typing, not about the spec's correctness or speed.

## Executive summary

**Verdict.** Eyelang is already pleasant to type for the programs it is for. A job that waits for
a timer and an event, an entity that owns state, a contract served over HTTP and called with
recorded fakes, and a model call with a typed answer and a tool each took one or two rounds from
first draft to passing tests. The equivalents elsewhere took more code and a framework. The
checker's messages about effects, steps and keys are the best part of the language to work with:
they say what is wrong, why, and what to write. Where Eyelang is worse to type is the ordinary
core, the part every program shares with a Python script: updating a map, sorting by a key, a
closure that reads a counter, a record built inside a lambda, slicing a string. There the obvious
first attempt often fails. The message is accurate but rarely names the fix, and a few messages
point at the wrong place. The editor helps with navigation but not yet with writing.

**The five best things, ranked**

1. **Errors that say what to write.** `in a job, Door(door).open reads Door without recording
   it, so a resumed run could see another value: read a snapshot with Door(door).read!() and
   take open from it`. Every rule of the durable core that I broke told me the exact edit. An
   agent can apply these without understanding the theory behind them.
2. **The durable core reads like a script.** The door alarm (an entity, a job that waits ten
   minutes for an event or a timeout, three tests on a test clock) passed on its second check;
   the only errors were two missing `uses` clauses, which `eye fmt` wrote. The Temporal version
   is 89 lines and 792 tokens against 72 and 516, needed its timestamps turned into strings
   before they would serialise, and its test clock drifted by 24 ms, so an exact assertion
   failed.
3. **Contracts remove a whole layer.** One 27-line contract gave a served `impl`, an HTTP server
   with auth, idempotency keys and status codes, a typed client with `.reconcile!()`, and fakes
   from a JSON recording. Apart from one slip in an entity (`this` for `self`), it worked first
   time, including over real HTTP with `curl`. The
   TypeScript version of the server and client alone is 1,044 tokens against Eyelang's 664 for
   everything, tests with recorded fakes included.
4. **The typed core is concise and safe where it counts.** List patterns over `Bytes` with
   character literals (`['-', '>', ..] => Arrow`), `?` that wraps errors, `let … else`,
   `name:` arguments, exhaustive matches, `Option` instead of null. The byte tokenizer passed its
   tests after one fix and is shorter than the Rust (841 tokens against 907).
5. **A fast, deterministic loop.** `eye check` takes 16 ms on a small program and 37 ms on the
   lab's 2,200 lines; the lab's 31 tests run in 0.28 s. A failed `expect` shows the value it got.
   Tests run on an exact clock with no flakes. `eye runs` shows a run's steps, and `eye repl`
   prints values with their types.

**The five worst things, ranked**

1. **Everyday collection code fails on the first attempt.** There is no `m[k] = v`, `sortBy`,
   `groupBy`, `update` or `maxBy`, and a closure can't read a `var` (G.13). The obvious word
   counter failed three ways before it checked, and the workarounds (`counts = counts |>
   insert(w, (counts[w] ?? 0) + 1)`, sorting tuples of negated counts) are the noisiest lines in
   the programs I wrote.
2. **Annotations are needed in places the messages don't point to.** A block body without `->`
   returns `()`, so leaving out a return type gives `expected (), found Option(a)` at a `?`
   deep inside. A record literal in a lambda bound by `let` has no type (G.14), and the error
   appears at a later field access. Type annotations on lambda parameters are parsed and then
   ignored (a bug), while the message says to write exactly the annotation that is already
   there.
3. **Two spellings and overloaded symbols.** `html |> indexOf("<")` and `html.slice(a..b)` sit on
   one line; the code says `bus.closed` and the signature says `uses {publish Bus.closed}`;
   `if !poll!(…)` puts both meanings of `!` in five characters. `@key(door)` on an event reads
   like "identified by door", but it is an idempotency key, so the second close of a door was
   silently dropped.
4. **A test can't read a run's result.** `run.join()` in a test passes the checker, then fails at
   run time with `*interp.RunVal isn't a collection`; `run.join!()` is refused. The workaround
   used throughout `testdata/programs` is an extra job that joins and an entity that records what
   it saw.
5. **The editor helps you read but not write.** There is no completion after `.` on a list or
   inside a lambda (`fn o: o.` gives nothing), no hover on a variable where it is bound, no
   inferred effects on a private function's hover (which SPECS promises), and no quick fixes for
   the many messages that already contain the fix. `eye fix` doesn't exist, `eye check` has no
   machine-readable output, and there are no warnings at all.

## The programs

| Program | What it exercises | Code lines | First check | Rounds to green | What tripped me |
|---|---|---|---|---|---|
| p1 log summariser | strings, maps, iteration, sorting, `fs`, `main` | 63 | 3 errors | 4 | map index assignment; no `sortBy`; `FsError` not in scope after `import fs`; `Level.parse` is case-sensitive (my bug, found by a test) |
| p2 tokenizer | `Bytes`, `Byte` and `UInt32`, character literals, list patterns, `mut self` | 93 | 4 errors, one cause | 2 | `'0'.toInt()`: a character literal is a `Char` unless a `Byte` is expected |
| p3 order pipeline | records, lambdas, `Result`, `?`, `mapErr`, `let … else` | 45 | 1 error | 2 | `map(fn c: {` for a lambda with a block body |
| p4 door alarm | entity, job, `next!` with a timeout, events, test clock | 72 | 2 errors | 2 | `uses` clauses (`eye fmt` wrote them); `@key` on an event silently dropped a repeat |
| p5 link shortener | contract with `@http`, `@auth`, `@key`, `@reconcile`; served `impl`; recorded fakes | 87 | 1 error | 2 | `this.code` (it is `self.code`) |
| p6 mail triage | `ai.generate!` with a typed result, a tool, `@budget`, recordings | 38 | 0 errors | 3 | `run.join()` in a test crashed at run time; `ai.generate!`'s result needs an annotation |
| p7 request stats | nested maps, record update, string positions (`Offset`) | 64, then 54 | 0 errors | 1 | nothing failed, but the `Offset` version is the most verbose code I wrote |

Two things stand out. The domain programs (p4, p5, p6), which I expected to be hard, were the
easiest. And none of the stumbles was about the language's big ideas; all were about names,
missing helpers and syntax habits.

## Writing

### Ceremony: what you must write

Counted over the lab's 1,621 lines of code (16 files, comments and blank lines left out):

| Mark | Count | Per 100 lines | Comment |
|---|---|---|---|
| `\|>` | 172 | 10.6 | the most common operator by far |
| `name!(…)` steps | 95 | 5.9 | every one is a place a run can pause; none felt noisy |
| `fn` lambdas | 48 | 3.0 | |
| attributes `@…` | 39 | 2.4 | mostly on jobs and contracts; they read like a spec sheet |
| `match` | 39 | 2.4 | |
| `var` | 27 | 1.7 | |
| type annotations on `let`/`var` | 28 | 1.7 | mostly empty collections and model results |
| `??` | 26 | 1.6 | half of them are `?? ""` after a string slice |
| `let … else` | 24 | 1.5 | |
| `?` | 20 | 1.2 | |
| `key:` arguments | 16 | 1.0 | |
| `mut` | 6 | 0.4 | |
| `uses` clauses | 5 | 0.3 | only public items need one, and `eye fmt` writes them |

The marks the language asks for to make effects visible (`!`, `uses`, `mut`, `key:`) add up to
about 7 per 100 lines, and most are `!`. They never felt like ceremony, because each one
answered a question a reviewer would ask. The ceremony I did notice came from elsewhere:

- **Signatures.** A private function may leave its types out, but in practice only one that
  does arithmetic can. One that reads a field, walks a collection or has a block body needs
  them. Of 99 named functions in `testdata/lab` and `testdata/programs`, only `impl`, trait and
  contract functions (whose types come from elsewhere) and two toy generics leave them out.
  Annotation counts per program were on par with TypeScript, Go and typed Python: 11 in p1
  against Python's 11 and Go's 9; 8 in p3 against TypeScript's 9 and Rust's 14.
- **Empty collections and model results.** `var steps: List(Step) = []` and `let briefing:
  Briefing = ai.generate!(…)` need their types written. That is reasonable, and the messages say
  so.
- **`Option` arguments.** `label: Some(…)`, as in `to: Some([me.id])`, appears 13 times in the
  lab: a parameter typed `Option(…) = None` must be given `Some(…)` (open question 31).
- **Public jobs.** Making `watch` public added `uses {consume Bus.closed, entity Door, entity
  Log, wait}`, and `eye fmt` wrote it as one 104-character line.

### How often I fought the checker, and why

Across the seven programs, nine things failed on first contact, counting causes rather than
sites. Five were missing helpers or habits from other languages, two were rules of the language
that the message explained well (the `uses` clauses), one was the spec's own example (`FsError`
after `import fs`), and one was a bug (`run.join()` in a test). Two more mistakes passed the
checker: a case-sensitive `Level.parse`, which a test caught, and an event `@key`, which only a
test I wrote afterwards caught. The rest of this section covers them, and what the probes found,
in order of how much they cost:

**Updating a map.** The first attempt is index assignment, which isn't allowed:

```eye
var counts = {}
for w in words { counts[w] = (counts[w] ?? 0) + 1 }      # error
```

```text
only a variable, a mut parameter, or a field of one, can be assigned
```

What works:

```eye
for w in words { counts = counts |> insert(w, (counts[w] ?? 0) + 1) }
```

The nested version in p7 is harder to read than it should be:
`stats = stats |> insert(r.user, mine |> insert(r.path, new))`. Python writes
`stats[r.user][r.path]` with a `defaultdict`.

**Sorting by a key.** There is `sort` and nothing else, so the top three words by count, ties
by word, became a trick with negated counts:

```eye
counts |> entries |> map(fn (w, c): (-c, w)) |> sort |> take(n) |> map(fn (c, w): (w, -c))
```

Python's `sorted(counts.items(), key=lambda kv: (-kv[1], kv[0]))[:n]` says the same thing
directly. Nothing in the lab sorts at all, which may be why this hasn't come up.

**A closure that reads a `var`.** The commonest loop shape there is:

```eye
var best = 0
for x in xs {
  if xs |> any(fn y: y > best) { best = x }   # a closure can't capture best, a var; copy it into a let first
}
```

The message is clear, and G.13 (decided) fixes exactly this.

**A record built in a lambda.** When the result flows straight to a return type it already
works, which surprised me pleasantly:

```eye
pub fn report(orders: List(Order), country: String) -> List(Row) {
  …
  customers |> map(fn c {
    …
    { customer: c, orders: theirs |> size, total: cents |> sum, average: cents.average() ?? 0.0 }
  })
}
```

But bound by a `let` first, it doesn't, and the error lands at the next use, not at the
literal:

```eye
let ls = orders |> map(fn o: { who: o.customer, cents: o.cents })
ls |> filter(fn l: l.cents > 0)
# the type of l isn't known here, so its field cents can't be found: write its type where it is declared
```

Writing the type where `l` is declared, `fn (l: Line): l.cents > 0`, gives the same error,
because lambda parameter annotations are ignored: `xs |> map(fn (s: String): s)` on a
`List(Int)` passes the checker. G.14 fixes the first half; the second is a bug with no task yet.

**Leaving out `->`.** A function with a block body and no `->` returns `()`. Leaving the return
type off a helper, as one does in a script, gives errors far from the cause:

```eye
fn parseLine(line: String) {          # meant: -> Option(Entry)
  let [at, level, ..rest] = line |> split(" ") else { return None }
  …
}
```

```text
main.eye:13:49: expected (), found Option(a)
main.eye:38:5: this match doesn't handle (): add an arm for it, or `_ => …`
```

The rule is sound (it saves `-> ()` on every method and job), but the message should name it.

**A lambda with a block body** is `fn c { … }`, not `fn c: { … }`. The second is the natural
mistake after writing `fn c: expr` many times, and gives `expected an expression before the end
of the line`.

**Character arithmetic.** `b.toInt() - '0'.toInt()` fails, since `'0'` is a `Char` unless a
`Byte` is expected. `'0'.code()` works. The message says what types `toInt` takes but not what
to write.

**The rules of the durable core cost almost nothing**, because their messages are exact:
`opened uses start watch, which its signature doesn't list: run eye fmt, or add uses {start
watch}`, and after extracting a function from a job, `alarmIfOpen waits, so calling it is a
durable step: write alarmIfOpen!(…)`.

### Symbols and words

Eyelang mostly picks words, and the words read well: `uses`, `then`, `match`, `unreachable`,
`allDone!`, `firstDone!`, `variants()`. The symbols that remain are universal (`?`, `??`, `|>`,
`..`), with two exceptions:

- **`!` means two things.** Logical not and "this is a step" meet in `if !poll!(every: 1 min,
  within: 6 h, fn { miniflux.me!() |> isOk })` in the digest. Each is unambiguous to the parser,
  but a reader has to look twice. `not` as a word would remove the collision; it is a small
  change with an `eye fix` rewrite.
- **`|>` and `.` for the same call.** Both work for built-ins today, and real code mixes them on
  one line: `html |> indexOf(">", from: i)?` next to `html.slice(start..i) ?? ""`. G.15 makes
  `.` work for every function, which helps writing; it makes reading worse unless one form is
  canonical, which `eye fmt` could enforce.

### How names read

Names are good: `split`, `words`, `lines`, `trim`, `startsWith`, `enumerate`, `reduce`,
`average`, `div`. The misses are the names a newcomer reaches for and doesn't find: `length` and
`len` (it is `size`), `fold` (it is `reduce`), `range(10)` (it is `0..10`), `sortBy`, `null` (it
is `None`). None of them gets a "did you mean", although `@concurency` does.

`key` carries three meanings: an entity's identity in `entity Link(key: Key)`, a map's keys, and
the idempotency key of `key:` and `@key`. I wrote `@key(door)` on a `Closed` event meaning
"which door it is about"; it made every later close of that door a repeat, dropped without a
word. The glossary explains the difference, but the attribute's name invites the mistake.

## Reading

I read the lab's `trains`, `access` and the first half of `digest` as someone who hasn't seen
them, and could follow them on the first pass. What makes them readable:

- **The top of a job is its contract with the runtime.** `@on(daily(07:40, in: dublin))`,
  `@deadline(8 h)`, `@budget(usd: 0.50)`, `@memory(256 MiB)`: what triggers it and what it may
  spend, before a line of the body.
- **Every pause is marked.** In `trains.watch`, the `!`s show at a glance which lines talk to
  Irish Rail, which change the entity, and where the job sleeps.
- **Failure is in the types.** `match … { Asking(q) => q; NotAsked(reason) => return Err(reason) }`
  shows every way a call can go, and an added variant shows up as an error at every match (I
  added `Throttled` to a contract result and got one message, at the one match).
- **Pure public functions are labelled by what's missing.** `pub fn plan(members, users) ->
  List(Step)` has no `uses`, so it is pure, and the checker holds it to that.
- **Comments are prose.** `##` comments read as documentation, and the module header comments
  explain intent, as the lab's do.

What is noise:

- **`?? ""`**, 13 times in the lab, nearly all after a string slice; the digest defines
  `fn past(s, i, by) = s |> after(i, by:) ?? s.end()` to cope. A position API that is safe for
  every call (`Option` everywhere) makes scanning code mostly fallbacks. My `Offset` version of a
  `key=value; …` splitter was 19 lines; the `split` version is 9.
- **Empty arms.** `None => ()` appears 5 times in the lab, the case open question 14 (`if let`)
  is about. Five is few; I would keep `match`.
- **Long `uses` lines** after `eye fmt`, since the formatter keeps one line however long.
- **Keys built from strings**, `key: "{dest.keys}ireland-{day}-failed"`. They are visible, which
  is the point, but nothing checks that two call sites don't build the same key.

What should be visible and isn't:

- **What a private function does.** `fn apply(step: Step)` in `access` calls NetBird and changes
  an entity; nothing at its signature says so, and the editor's hover doesn't either.
- **That an event is deduplicated.** Nothing at `bus.closed.publish(…)` or in the test output
  says a publish was dropped as a repeat.

## Changing code

Each refactor below was done on the door alarm (p4), the link shortener (p5) or a small
program with a journal.

| Change | What it cost | Help from the tools |
|---|---|---|
| Extract part of a job into a function | one error: the call needs `!`, since the function now waits | the message gives the exact edit |
| Make a job public | a four-entry `uses` clause | `eye fmt` wrote it, on one long line |
| Add a variant to a contract's result | one error per `match` that doesn't handle it | exact, at each match |
| Change an entity field from `Bool` to a variant, named `state` | nothing: `state state: Position = Shut` works, since `state` is contextual | |
| Rename an entity field with stored rows | nothing at all, and that is the problem: the stored values are dropped | none (see below) |
| Add an effect deep in a call chain | one error at each public function above it | `eye fmt` writes the clauses |
| Add a field with a default | nothing | |

**Renaming a stored field loses its data silently.** With a journal holding `Door("x")` at
`open: true`, renaming the field to `position: Int = 0` and running again printed `pos: 1`: the
old value was dropped and the default used, with no warning. SPECS says this ("a field the entity
no longer declares is dropped"), and it is what makes adding fields free, but a rename is the
commonest change of all, and the migration that would have kept the value is easy to forget.
`eye run` already reads every stored row's version before it starts; it could also say which
stored fields the program no longer declares.

**`eye fmt`** is a real help for effects and a good formatter otherwise: it is idempotent, has no
options, and fixed a deliberately messy file in one pass. It keeps one-line `match` and both
forms of `if` as written (open question 13), and never wraps a line.

**`eye fix`** is named in CLAUDE.md as the way to keep old code working across language changes,
but there is no such command yet. G.12 (results must be used), G.15 (methods) and any change to
`!` or `key` will want it.

## Error messages

About forty messages were collected from real mistakes; thirty-two are listed. Ratings: **A** clear and
actionable (says what to write), **C** clear (says what is wrong), **?** confusing, **W** wrong
span or wrong advice. Paths are shortened.

| # | Code | Message | Rating | Better |
|---|---|---|---|---|
| 1 | `size!(xs)` in a job | `size neither waits nor reaches outside the program, so it isn't a durable step: write size(…) without !` | A | |
| 2 | `Door(door).open` in a job | `in a job, Door(door).open reads Door without recording it, so a resumed run could see another value: read a snapshot with Door(door).read!() and take open from it` | A | |
| 3 | `watch.start("front")` in `main` | `eye run runs main every time it starts, so this would start watch again after each restart; give the run a key, which starts it once: watch.start(…, key: "…")` | A | |
| 4 | `pause()` in `main`, where `pause` sleeps with `!` | `pause has durable steps, which only a job records, so main can't call it: start a job that does` | A | |
| 5 | a loop over an `Iter` that waits, in a job | `this loop walks an Iter, which can't be saved, and its body waits, so a resumed run couldn't find its place: collect it with toList before the loop` | A | |
| 6 | `pub fn opened(…)` that starts a job | `opened uses start watch, which its signature doesn't list: run eye fmt, or add uses {start watch}` | A | |
| 7 | `@concurency(1)` | `@concurency isn't an attribute; did you mean @concurrency?` | A | the model for the rest |
| 8 | `record(c)` where `record` takes `mut c` | `record changes its argument c, so the call says so: write mut c` | A | |
| 9 | a `match` missing a variant | ``this match doesn't handle Tri(_, _, _): add an arm for it, or `_ => …` `` | A | |
| 10 | `let Some(v) = x` | ``this pattern doesn't match None: add `else { … }` for what it doesn't match`` | A | |
| 11 | a fake with no recorded answer | `…links.json has no exchange for links.shorten with key "batch-2026-10-12-https://b.example"` | A | list the keys it does have |
| 12 | `Door(door).read()` in a job | `in a job, an entity call is a durable step: write read!(…)` | A | `write Door(door).read!()` |
| 13 | `let x = { name: "Ann", age: 3 }` | ``which record type is this? Nothing here says; write it where the value is bound, as `let x: T = …` `` | A | |
| 14 | a closure reading a `var` | `a closure can't capture best, a var; copy it into a let first` | C | gone with G.13 |
| 15 | `counts[w] = …` on a `var` map | `only a variable, a mut parameter, or a field of one, can be assigned` | C | `a map isn't assigned by key: write counts = counts \|> insert(w, …)` |
| 16 | `'0'.toInt()` | `toInt takes a Float or a fixed-width integer (Byte, Int32, Int64, UInt32 or UInt64), and this is Char` | C | add `: a character's code is '0'.code()` |
| 17 | `x / y` with `y: Int` | `dividing by Int fails when it is 0, so / and % take a NonZero(Int): match nonzero(y), …` | C | lead with `x.div(y)`, an `Option`, since G.16 |
| 18 | `xs \|> first?` in a function returning `Result` | `? on an Option needs a function that returns an Option, and this one returns Result(Int, String)` | C | add `: write let Some(f) = … else { return Err(…) }` |
| 19 | `fold(0, …)`, `s.length()`, `xs.len()`, `range(10)`, `null` | `undefined name fold`, `String has no method length`, … | C | `did you mean reduce / size / 0..10 / None?` |
| 20 | `{ nmae: name, age: 1 }` | `this User has no name: every field needs a value` and `User has no field nmae` | C | one message: `User has no field nmae; did you mean name?` |
| 21 | `"hello".shout()`, `shout` a function | `String has no method shout` | C | until G.15: `shout is a function: write "hello" \|> shout` |
| 22 | `Ok()` | `Ok needs an argument for its argument 1` | ? | `Ok takes a value; for nothing, write Ok(())` |
| 23 | `map(fn c: {` with a block | `expected an expression before the end of the line` | ? | `a lambda with a block body is fn c { … }, without the colon` |
| 24 | `User { name: "Ann", age: 3 }` | `expected a line break or ';', found '{'` | ? | `a record literal has no type name: write { name: …, age: … } where a User is expected` |
| 25 | `xs.filter(\|x\| x > 0)` | `expected an expression, found '\|'` | ? | `a lambda is fn x: x > 0` |
| 26 | `Result((), FsError)` after `import fs` (as in SPECS section 13) | `undefined type FsError` | ? | `FsError is declared in fs: write fs.FsError, or import fs.{FsError}` |
| 27 | a recorded model reply that doesn't match the schema | `the recordings for ai have no reply left for this call` | ? | say that the reply before it didn't match, and why |
| 28 | `now()` in an entity method | `touch uses clock, which its signature doesn't list: run eye fmt, or add uses {clock}`, then the right message | W | drop the first: adding `uses {clock}` can't fix it |
| 29 | `n / 2.0` with `n: Int` | `expected Float, found Int`, then `dividing by Float fails when it is 0, so / and % take a NonZero(Int): match nonzero(2.0), …` | W | Float division doesn't need `NonZero`; say `toFloat(n) / 2.0` |
| 30 | `fn (l: Line): l.cents`, or a record built in a `let`-bound lambda | `the type of l isn't known here, so its field cents can't be found: write its type where it is declared` | W | honour the annotation; point at the record literal that has no type |
| 31 | a block body with no `->` that ends in an `Option` | `expected (), found Option(a)` at a `?` inside | W | `parseLine has no ->, so it returns (); write -> Option(Entry)` |
| 32 | `run.join()` in a test | passes the checker, then at run time `*interp.RunVal isn't a collection` | W | a bug: see recommendation 1 |

Tally: 13 A, 8 C, 6 ?, 5 W. The messages for effects, steps, keys, `mut` and exhaustiveness are
as good as any language's. The weak ones share three causes: no "did you mean" outside
attributes, no recognition of other languages' syntax, and type errors reported where a type
was needed rather than where it was lost.

Two more things about how errors arrive:

- **They come in rounds.** Name resolution stops the check, so `undefined name fold` hid every
  type error in the file, and a file of seven mistakes took three runs to see them all
  (syntax, then names, then types). Fine for a person; an agent pays a round trip per phase.
- **There are no warnings.** An unused `let`, a `for i` that never reads `i`, and an arm after a
  match that already covers every case (`_ => 0.0` after `Circle` and `Square`) all pass
  silently.

## Editor and tools

**The language server** (`eye lsp`, driven by its flags and by a small JSON-RPC client):

| Request | What it did |
|---|---|
| Go to definition, references | correct, including through entity references (`Door(door).alarm!()` to `pub fn alarm()`) |
| Diagnostics | live on the unsaved buffer, the same messages as `eye check` |
| Hover on a use | the type, `heard: Option(Closed)`, `o: Order`, or a function's signature and `##` docs |
| Hover where a name is bound | nothing, at `let heard = …`, `let d = …`, a pattern's `Some(e)`, or inside `"{e.at - since}"` |
| Hover on a private function | its signature, but not its inferred effects, which SPECS section 12 says editors show |
| Completion after `Door(door).` | the entity's state fields and methods with signatures: good. It offers state fields in a job, where reading one is an error |
| Completion after `mine.` (a list) or `\|> ` | 218 items: every name in scope and the keywords, not the collection functions |
| Completion after `fn o: o.` | nothing, since the line doesn't parse yet |
| Rename, code actions, signature help, inlay hints | not implemented |

Completion inside a lambda's body is the place pipeline code needs it most. And many messages
already contain their fix (`write mut c`, `write read!(…)`, `add uses {start watch}`, `add an
arm`): each is one code action away from a click.

**`eye fmt`**: see Changing code. Fast, predictable, and it writes `uses` clauses.

**`eye test`**: concise, shows the failing value, `-v` shows what passing tests printed and
`--run` picks tests. Tests are deterministic: the same clock, seed and empty directory every
time.

**`eye runs`**: one line per run (`#1 tick("a")  sleeping until 2026-10-12 13:11:41 (in 59m58s)
[2 steps, 469 B]`), and a run's steps by id. Exactly what someone checking on a job wants.

**`eye repl`**: prints each value with its type, `:type` and `:doc` work. Good for finding
function names, the job the editor doesn't do yet.

**`eye doc`** writes clean pages, but its default is dangerous: it writes to `api/` in the
current directory and replaces that directory. Run in a Go or TypeScript repository, it deletes
the `api/` folder. A test with a file in `./api` confirmed it: the file was gone.

**`eye check`**: fast (16 to 40 ms here), file:line:column messages, exit status 1. No `--json`
and no suggested edits in a form a tool could apply.

What's missing for a pleasant loop, in order: completion that knows the type before the cursor
(after `.`, after `|>`, in lambdas), quick fixes for the messages that name their fix, hover on
bindings and private effects, warnings, and `eye check --json` with edits.

## Agents as users

**Easy to get right the first time:**

- The durable core. The messages for `!`, entity reads in jobs, keys from `main`, `Save` across
  steps and `Iter` in loops each name the exact edit, so an agent converges in one round
  without knowing why the rule exists.
- `uses` clauses: run `eye fmt DIR` and they are correct.
- Exhaustive matches, `mut` at the call, `let … else`: the message says what to add.
- Contracts and recordings: the JSON shape is regular, and a missing exchange names the
  operation and the key.

**Easy to get wrong:**

- **Habits from other languages**, which an agent brings by default: `|x|`, `x.len()`,
  `User { … }`, `if let`, `null`, `range(n)`, `fold`, `sortBy`, `m[k] = v`, `Ok()`, `?:`. Each
  gives a message that says what's wrong but not what to write, so the agent guesses again.
- **The spec's own examples**: the `main` in SPECS section 13 writes `FsError` after
  `import fs`, which doesn't check (C.7 covers examples that don't).
- **Silent semantics**: an event `@key` that drops repeats, a renamed field that drops data, a
  `Result` dropped as a statement (`fs.writeText("out.txt", "hi")` alone passes; G.12). No
  message means no correction.
- **Two forms for one thing**: `|>` or `.`, `if … then` or `if … { }`, a `uses` entry spelled
  `Bus.closed` for code that says `bus.closed`. An agent copies whichever it saw last, and diffs
  get noisy.

**What would make it better**, roughly by value:

1. Messages that end with the code to write, for the twenty commonest foreign habits and
   misspellings (a table in the checker, as attributes already have).
2. `eye check --json`, with each diagnostic's span and, where the message names a fix, the edit
   itself, so an agent applies fixes without parsing English.
3. One canonical form per construct, enforced by `eye fmt` (which `if`, which call form).
4. A page "Eyelang for Python, TypeScript and Rust programmers" mapping the habits above to
   their Eyelang form, linked from the errors. Agents read docs when pointed at them.
5. No silent drops: say in test output when an event was dropped as a repeat, and refuse or warn
   when stored fields would be lost.

## Comparison

Code lines leave out blank lines and comments; tokens count identifiers, literals and operators;
annotations count types written in signatures and local bindings (roughly, by pattern, and not
record fields). Every version was run to passing tests, the Temporal one against Temporal's
time-skipping test server.

| Program | Eyelang | Other | Notes |
|---|---|---|---|
| p1 log summariser | 63 lines, 530 tokens, 11 annotations | Python 74, 499, 11; Go 117, 693, 9 | Python's `Counter` and `sorted(key=…)` do in one line what Eyelang does in four |
| p2 tokenizer | 93, 841, 17 | Rust 109, 907, 10 | near equal; Eyelang has no lifetimes, Rust has `while let` and `is_ascii_digit` |
| p3 order pipeline | 45, 456, 8 | TypeScript 58, 698, 9; Rust 75, 711, 14 | Eyelang's `Result`, `?` and `let … else` beat TypeScript's hand-written result type |
| p4 door alarm | 72, 516, 5 (3 tests) | Python + Temporal 89, 792, 4 (2 tests) | Temporal needed ISO strings for times, two-step "update the row, then signal", and its clock drifted 24 ms |
| p5 link shortener | 87, 664, 8 (contract, server, client job, 2 tests with recorded fakes) | TypeScript on Bun 88, 1,044, 7 (server, client with reconcile, 1 test, no fakes, no shared contract) | the contract replaces routing, validation, auth, a key table and the reconcile loop |
| p7 request stats | 54, 512, 8 (64, 602 with `Offset`s) | Python 59, 403, 6 | `defaultdict` and `str.partition` against `insert` and `split` |

**Which comparisons matter.** p4 and p5 matter most: they are the programs Eyelang is for, and
there it is clearly ahead, in size (a third fewer tokens) and more in what you don't write
(serialisation, idempotency, routing, a test clock, fakes). p2 and p3, against Rust and
TypeScript, show the typed core holds its own with typed languages. p1 and p7, against Python,
are where Eyelang loses, by 6 to 27% in tokens, and every bit of that loss is in map updates,
sorting and string slicing: the gap is in the standard library and two syntax rules, not in the
design. Annotations were never the difference: Eyelang writes as many as TypeScript at function
boundaries and none for locals.

## Recommendations

Ranked by value for cost. "Filed" names an existing ROADMAP task or decision; "new" is not filed.

| # | Change | Problem it fixes | Example | Cost | Status |
|---|---|---|---|---|---|
| 1 | Let a test read a run's outcome, and fix `run.join()` | `run.join()` checks in a test and then crashes with a Go type name; `join!` is refused; tests write a joining job and a log entity instead | `let r = sort.start(m); advance(1 min); expect(r.outcome() == Some(Completed(Ok(f))))`, or make `r.join()` block as SPECS says waits do outside jobs | small | new (bug) |
| 2 | Honour lambda parameter annotations, and report an untyped record literal at the literal | the advice "write its type where it is declared" fails when followed; a wrong annotation passes | `fn (l: Line): l.cents` types `l`; `let ls = xs \|> map(fn o: { … })` reports at the `{` | small to medium | new (bug); the literal half is G.14 |
| 3 | `m[k] = v` and `m[k] += x` on a `var` map, as `insert` | the first attempt at counting or grouping fails | `counts[w] = (counts[w] ?? 0) + 1` | medium: a place can end in an index; both parsers, the checker | new |
| 4 | `sortBy`, `minBy`, `maxBy`, `groupBy`, `count` by key | sorting by a field needs a tuple trick | `counts \|> entries \|> sortBy(fn (w, c): (-c, w))` | small: built-ins | new |
| 5 | "Did you mean" for names, fields, methods and arguments, plus a table of foreign habits | 6 of 32 messages left me guessing | `undefined name fold; did you mean reduce?`, `a lambda is fn x: x > 0` | small, grows with the table | new |
| 6 | Fix the wrong advice | three messages send the reader the wrong way | `n / 2.0` asks for `nonzero(2.0)`; `now()` in a method says to add `uses {clock}`; `x / y` doesn't mention `x.div(y)` | small | new |
| 7 | Name the missing `->` | leaving out a return type gives errors far from the cause | `parseLine has no ->, so it returns (); write -> Option(Entry)` | small | new |
| 8 | Callbacks read a `var` | the commonest loop shape fails | `if xs \|> any(fn y: y > best)` | decided | G.13 |
| 9 | Record literals typed from where the result goes | records built in lambdas need a `let` with a type | `let ls = orders \|> map(fn o: { who: o.customer })` | decided | G.14 |
| 10 | Results can't be dropped silently | `fs.writeText(…)` alone loses its error | `let _ = fs.writeText(…)` or `?` | decided | G.12 |
| 11 | Any function as a method, then one canonical form | `x.f()` works for built-ins only; lines mix `\|>` and `.` | `"hello".shout()`; `eye fmt` picks one form per call | decided, plus a formatter rule | G.15; the canonical form is new |
| 12 | Make `eye doc` safe | it replaces `./api` by default | write to `docs/api` or stdout, and refuse a directory it didn't write | tiny | new |
| 13 | Say when stored fields will be dropped | renaming an entity field loses its values silently | `eye run: 1 Door row holds open, which Door no longer declares: add @version(2) and migrate from 1 to keep it, or --drop-fields` | small; reopens part of the "Evolution" decision | new |
| 14 | Name event deduplication for what it is, and show it | `@key(door)` dropped every later close of a door | `@dedupe(id)`, or a line in `eye test -v`: `bus.closed {door: side} dropped: key side was published at 09:01` | small | new |
| 15 | Completion that knows the type: after `.`, after `\|>`, inside a lambda; hover at bindings; private functions' effects on hover | the editor doesn't help write pipeline code; SPECS promises the effects | `fn o: o.` lists `Order`'s fields | medium | new |
| 16 | Code actions for the messages that name their fix | each fix is typed by hand | "add `uses {start watch}`", "write `mut c`", "add the missing arms" | small per action | new |
| 17 | `eye check --json` with edits, and `eye fix` | agents parse English; CLAUDE.md relies on `eye fix` for language changes | `{"span": …, "message": …, "edit": {"replace": "mut c"}}` | medium | new |
| 18 | Warnings, or errors, for unused bindings and unreachable arms | dead code passes silently | `_ => 0.0` after a match that covers every case | small, plus a policy decision (the formatter has no options; should the checker have warnings?) | new |
| 19 | String helpers that don't return `Option` for the common case | 13 `?? ""` in the lab; scanning code is mostly fallbacks | `s.splitOnce("=")` giving `Option((String, String))`; a clamping `s.sliceOr(a..b)`; `padStart`; a fixed-decimals `Float` format | small | new |
| 20 | Spell `uses` entries as the code does | `uses {publish Bus.closed}` beside `bus.closed.publish(…)` | accept and write `publish bus.closed` | small, with `eye fix` | new |
| 21 | Wrap long signatures in `eye fmt` | a public job's `uses` clause makes a 104-character line | break after `uses {` at 100 columns | small | new |
| 22 | `?` on an `Option` outside an `Option` function | | | filed | G.17 |
| 23 | `not` for logical negation | `if !poll!(…)` | `if not poll!(…)`, rewritten by `eye fix` | small, with `eye fix` | new; low priority |

**What I would not change.** `!` on steps, `uses` at the boundary, `mut` at the call, keys on
commands, exhaustive matches, attributes on jobs, `then` for one-line `if`, and `match` instead
of `if let` (five empty arms in the whole lab). Each of these cost me at most one round, with a
message that said what to write, and each answered a question I would ask in review.
