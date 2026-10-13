---
title: Failure paths and pipes, 2026-10-13
---

# Failure paths and pipes, 2026-10-13

A review of how Eyelang writes "check this, or fail with that", and of what `|>` is for now that
any function is a method. The owner asked for it after reading the errors tutorial: it "looks too
much like JS", the pipe in `text |> trim` is redundant, and the checks don't fold into the `let`
they guard.

Written against `dev` at a829474. Everything marked **tested** was checked with `eye check` and
`eye test` built from that commit. The counts come from a scan of the 315 `.eye` files in the
repository; "real code" means the lab, docs, programs, benchmarks and the compiler's tests, but
not `testdata/syntax`, `testdata/fmt` or `ir/builtin.eye`. Treat the counts as accurate to a few.

**Timing.** The native-focus checkpoint (top of ROADMAP's What's next) parks new syntax until
v0.4 unless the owner decides otherwise. Everything below is a proposal for the owner, including
when to do it. Section 5.1 changes no syntax and can be done now.

## The owner's questions

| # | Question | Answer in short | Section |
|---|---|---|---|
| 1 | Why does `parseQuantity` read like JS, and where did we go wrong? | Half is the tutorial: two `if` forms, a needless pipe, a wrong reason for `mapErr`. Half is the language: no short "or fail with X". | 2, 4 |
| 2 | Why can't `if t == "" { return Err(Empty) }` fold into the first line? | Nothing attaches a check to a binding. `ensure` would; decide it on the lab rewritten with 5.2 and 5.3. | 4.3, 5.5 |
| 3 | Why can't the error mapping and the `n > 99` check fold into `let n`? | `?` can't take an error. With 5.2 the mapping folds in: `parseInt(t) ? NotANumber(t)`. The range check needs 5.5. | 4.1, 5.2, 5.5 |
| 4 | `\|> trim` is redundant: should pipes be more ergonomic, or change in scope? | Yes. 85% of pipes are one stage, and G.15 made `\|>` a copy of `.`. Make `.` the call, and give `\|>` the job of feeding a value into any expression. | 4.5, 5.6 |
| 5 | What are calls without parentheses for functions with no arguments? | `text.trim.lower` for `text.trim().lower()`. It is safe because a type can't have a field and a method of the same name. | 5.6 (P1) |
| 6 | How does `?` work? | It unwraps `Ok` or `Some`, or returns the `Err` or `None` at once, wrapping the error when exactly one case of the caller's error type holds it. | 3, 4.1 |
| 7 | Can `?` take an argument, as `mapErr` does? | Yes, after a space: `? NotANumber(t)` or `? BadNumber`. `?(…)` is taken: it calls the unwrapped value. | 5.2 |
| 8 | Why do we need braces around `return`? | We don't: the Blocks rule already drops them for one statement, except in `let … else`. Allow `else return …`, and make the `then` form the one-line `if`. | 4.2, 5.3 |
| 9 | Could we write `task.send "joko", 24 h`, without `()`? | Only at the start of a statement, with the arguments running to the end of the line. Elsewhere it collides with indexing, minus, tuples, records and unit literals. Decide after P1. | 5.7 |
| 10 | Could a leading dot mean an error, `.NewErr`? | No. Variants are already bare names, a line that starts with `.` continues the one before, and the symbol isn't universal. `? x` and `fail x` give the same brevity in words. | 5.8 |

## 1. The short version

```eye
fn parseQuantity(text: String) -> Result(Int, ParseError) {
  let t = text |> trim
  if t == "" { return Err(Empty) }
  let n = parseInt(t).mapErr(fn _: NotANumber(t))?
  if n > 99 then return Err(TooMany(n))
  Ok(n)
}
```

Five lines express one idea, "check the input or fail with this error", and use four mechanisms
for it: a pipe, a braced `if`, `mapErr` with a lambda that ignores its argument, and an `if …
then`. About half of the problem is the tutorial, and half is the language:

- **The tutorial.** It uses two forms of `if` three lines apart, a pipe where a call would do, and
  a reason for `mapErr` that is wrong (section 2). A version that is clearly better already
  parses and passes the tutorial's tests (section 3).
- **The language.** Eyelang has no way to attach "or fail with X" to a value:
  - `?` can't take the error to fail with;
  - `let … else` must spell out `{ return Err(…) }`, braces included, though the rest of the
    language drops braces for one expression;
  - `|>` now does what `.` does, so a call has two spellings and `|>` has no job of its own.

Recommended, in order:
1. Fix the tutorial now.
2. Decide open question 13 (one form of `if`) and one form per call.
3. Then `?` with an argument, and `let … else` without braces.
4. Then the pipe changes.

Section 6 has the plan.

## 2. The example, line by line

| Line | What is wrong | Kind |
|---|---|---|
| `let t = text \|> trim` | A one-stage pipe with no arguments is `trim(text)` with more punctuation. `parseInt` already ignores surrounding spaces (`ir/builtin.eye`: "Spaces around it are ignored"), so `t` exists only for the `""` check and the error. | tutorial; pipes have no job (4.5) |
| `if t == "" { return Err(Empty) }` | The check sits apart from the binding it checks. It is the braced form of `if`; two lines down the same guard uses `then`. | tutorial; two `if` forms (4.4) |
| `parseInt(t).mapErr(fn _: NotANumber(t))?` | A lambda that ignores its argument, only to build a constructor, then `?`. The point is "unwrap, or fail with `NotANumber(t)`", and `?` has no way to say that. | language (4.1) |
| `if n > 99 then return Err(TooMany(n))` | The second spelling of the same guard. `return Err(…)` is three tokens of ceremony to say "fail". | tutorial; language (4.2, 4.4) |
| `Ok(n)` | Fine. | |

The page's explanation is also wrong. `errors.md` says to write `mapErr` "when there is no such
case, or more than one … as `parseQuantity` does". But `ParseError` has exactly one case that
holds a `String`, `NotANumber(String)`, so a plain `parseInt(t)?` would wrap the error by itself.
The `mapErr` is there only to put the input `t` in the error in place of `parseInt`'s message.
Without it, the result would be `NotANumber("not a whole number: \"three\"")`.

## 3. What already works (tested)

Three versions of `parseQuantity` that pass the tutorial's four expectations today. The first
uses `let … else`, which has been in the language since §9:

```eye
fn parseQuantity(text: String) -> Result(Int, ParseError) {
  let t = text.trim()
  if t == "" then return Err(Empty)
  let Ok(n) = parseInt(t) else { return Err(NotANumber(t)) }
  if n > 99 then Err(TooMany(n)) else Ok(n)
}
```

The second is one expression, nested `match` with an arm guard:

```eye
fn parseQuantity(text: String) -> Result(Int, ParseError) = match text.trim() {
  "" => Err(Empty)
  t => match parseInt(t) {
    Err(_) => Err(NotANumber(t))
    Ok(n) if n > 99 => Err(TooMany(n))
    Ok(n) => Ok(n)
  }
}
```

The third matches on a tuple:
`match (text.trim(), parseInt(text)) { ("", _) => …; (t, Err(_)) => …; (_, Ok(n)) if n > 99 => …; (_, Ok(n)) => Ok(n) }`.

Also confirmed:

- `.mapErr(NotANumber)` works. A constructor is a function value, and four of the seven
  `.mapErr(…)?` in real code pass a bare constructor.
- `let Ok(n) = e else return …` without braces is a syntax error: `expected '{', found 'return'`.
- `?` on an `Option` in a function that returns a `Result` is an error. The message suggests `??`,
  a `match`, or returning an `Option`. Turning a `None` into an `Err` takes `let … else` or a
  `match`.
- `x?(…)` already means "unwrap, then call the result", so `?(f)` can't be the spelling for "map
  the error".
- `text.trim` without parentheses is `String has no field trim`.
- There is no `guard`, `unless`, `ensure`, postfix `if`, `okOr`, `orElse` or `unwrapOr`.

## 4. Where the design went wrong

Each decision below was reasonable where it was made. Together they leave the failure path with
no short form of its own.

### 4.1 `?` is all or nothing

`?` either passes the error on unchanged or wraps it, when exactly one case of the caller's error
type holds its type (decision log, "Errors"). Anything else goes through `.mapErr(…)`. Real code
calls `mapErr(…)?` only 7 times, but all three lambda forms are in the docs, the place people
learn from. The error-mapping lambda also costs something no count shows: it makes "fail with
this" look like functional plumbing.

`?` also can't turn an `Option` into an `Err`, which the 2026-10-12 ergonomics review found as
error message 18. Today that takes a `let … else`.

### 4.2 `let … else` asks for braces and `return Err(…)`

The decision log's "Blocks" row says: "Braces, only where they hold several statements. A branch
that is one expression or one assignment needs none." `let … else` is the exception: its
grammar is `[ "else" block ]`. So the commonest failure line is:

```eye
let Some(chat) = prefs.chat else { return Err(NoChat) }
```

That line has four tokens of ceremony: `{`, `return`, `Err(` and `}`. There are 76 `let … else`
in real code: 16 end `{ return }`, 4 `{ return Err(…) }`, 2 `{ return None }`, 17 hold a
multi-line block, and most of the rest are `{ unreachable(…) }` in tests. The `else` block also
can't see the error it caught: there is no `Err(e)` binding in it, so it can't map the error.

### 4.3 Checks can't attach to a binding

A condition on a value always becomes a separate `if … return`. Real code has 60 one-line
`if c { return … }` guards and 5 more over several lines. 26 block-bodied functions (about 6%)
have two or more `return`s, and about 7 have the full parseQuantity shape: a guard, a `?`, then
`Ok(…)`. They cluster in the lab's command handlers (`notifier.press`, `deliver`,
`digest.command`) and in the docs, which is why newcomers see the shape at once.

### 4.4 Two forms of `if`, by accident

`if c then x` exists for one-expression branches. `if c then return x` works because the Go
parser and `grammar.js` made `return`, `break` and `continue` expressions. The §23 sketch still
lists `return` only as a statement, and `then` is missing from §4's keyword lists.

Open question 13 asks whether `eye fmt` should rewrite a one-line `if c { a }` into the `then`
form. It is undecided, and `eye fmt` rewrites neither. The result in real code:

- 60 one-line brace guards and 7 `then return`, mixed within single functions;
- `trains.eye`'s `delayNews` uses all three guard shapes in 8 lines.

### 4.5 `|>` lost its job when every function became a method

G.15 (2026-10-12) made `x.f(a)` mean `f(x, a)` for any function, and kept pipes. The decision
log says to "write whichever reads better", and `eye fmt` rewrites neither form. This is the
opposite of principle 4 ("one obvious way to do each thing") and principle 10 ("canonical
formatting"). The ergonomics review recommended one canonical form (its recommendation 11).

What the code does with pipes now:

| Measure (real code) | Count |
|---|---|
| `\|>` | 624 |
| Chains of one stage | 85% (451 of 525) |
| Stages with no arguments (`\|> size`, `\|> trim`, `\|> sort`) | 251 |
| Uses of `_` to place the value | 1 (outside syntax tests) |
| Pipes into anything but a named call (lambda, `match`, `if`, constructor) | 0 |
| Lines that mix `\|>` and `.method()` | about 51 |
| `x = x \|> f(…)` updates | 24 (plus 18 `x = x.f(…)`) |

So the pipe is used almost only as a method call that needs no `()`. The one thing only it can
do, placing the value with `_`, goes unused. The same function is spelled both ways depending on
the file: `size` is `|> size` in the lab and docs, but `.size()` in the compiler's tests and
benchmarks. `docs/examples/log-summary/main.eye:89` has three styles on one line:
`s.chars() |> toList() |> size`.

### 4.6 Small lambdas are written out in full

There are 71 lambdas that only read a field (`fn u: u.name`, 43) or compare one
(`fn p: p.id == id`, 28). These aren't about failure, but they are the same kind of ceremony, and
they are most of what goes inside a pipe.

## 5. Options

Each option says what it would read like, what it costs, and what it decides. The examples in
5.2 to 5.6, and the code after them in section 6, is proposed syntax shown as plain text: it
doesn't parse today.

### 5.1 Fix the tutorial (no syntax change; can be done now)

Rewrite `parseQuantity` in `docs/tutorial/code/errors/main.eye` as the first version in section
3, and correct `errors.md`'s explanation of `mapErr`. Show both ways it is used: a bare
constructor (`.mapErr(BadNumber)?`) to keep the original error, and `let … else` to replace it.
The program is a playground example, so `TestDocsPrograms` and the playground's end-to-end test
run it.

- **Cost:** small. **Gain:** the page stops teaching the mixed form.
- **Limit:** it doesn't answer the owner's real question. The guard still can't fold into the
  `let`.

### 5.2 `?` takes an argument

Suggested by the owner: let `?` take what `mapErr` takes. The argument is either the error to fail
with, or a function from the old error to the new one:

```text
let n = parseInt(t) ? NotANumber(t)       # fail with this error
let n = parseInt(t) ? BadNumber           # a function: BadNumber(message), as .mapErr(BadNumber)?
let chat = prefs.chat ? NoChat            # Option to Result: None fails with Err(NoChat)
let port = parseInt(s) ? fn e: BadPort(s, e)
```

**Rules.**
- `e ? x` is `e?` when `e` succeeds.
- When it fails, the function returns `Err(x)`. If `x` is a function that takes the error, it
  returns `Err(x(error))`.
- On an `Option`, `None` returns `Err(x)` in a function that returns a `Result`, and `x` must be a
  value. It returns `None` in a function that returns an `Option`, where an argument is an error.
- The argument must start on the same line as `?`. A line that starts with `?` continues the line
  before (§4), so a lone `?` at the end of a line stays the plain `?`.
- The argument is a postfix expression (a name, a call, a constructor), or a lambda. Anything
  wider needs parentheses.

**Gains.**
- It removes every `mapErr(…)?` and the lambdas that ignore their argument.
- It fills the `Option`-to-`Result` gap that ergonomics error 18 found.
- It replaces the commonest `let … else { return Err(…) }` lines.
- It is the "fold the error into `let n`" the owner asked for.

**Costs.**
- **Readers from C and JS may see a ternary.** Eyelang has no ternary, and the argument always
  names an error, which helps.
- **The function-or-value rule is type-directed.** It is ambiguous only when the error type
  itself is a function type, which no program has; that case can be an error that asks for
  `fn e: …`.
- **`?(f)` isn't available.** It already means calling the unwrapped value, so the argument
  follows a space.

**Alternatives considered.**
- `e ?: X` — symbol soup.
- `e? else X` — longer, and `else` already belongs to `if` and `let`.
- Library helpers `orErr(x)` and `okOr(x)`, as in `parseInt(t).orErr(NotANumber(t))?`. These
  need no syntax and still read like plumbing. CLAUDE.md's rule ("clearly better than a library
  function") is the test, and `? NotANumber(t)` passes it.

### 5.3 `let … else` without braces, and one form of `if`

Suggested by the owner: "why do we need curly brackets around return". The Blocks rule already
answers that one statement needs none. Apply it to `let … else`:

```text
let Some(chat) = prefs.chat else return Err(NoChat)
let [_, method, path, status, ms] = fields else return Err(WrongFields(found: fields.size))
let Some(u) = users[0] else return "nobody"
```

The grammar becomes `"else" ( block | branch )`. The branch must still leave, as #120 requires.
At the same time, decide open question 13: a one-line `if` is the `then` form, braces are for
several statements, and `eye fmt` rewrites `if c { return x }` into `if c then return x`.

- **Cost:** small. The grammar change touches `grammar.js`, `syntax/`, the corpus and
  `testdata/syntax`, plus a formatter rule.
- **Gain:** one form for every one-line guard, and the 60 brace guards, 76 `let … else` and 7
  `then return` lines become one family.

### 5.4 `fail`: say "fail" instead of `return Err(…)`

A contextual keyword:
- `fail x` is `return Err(x)` in a function that returns a `Result`;
- bare `fail` is `return None` in one that returns an `Option`.

```text
if n > 99 then fail TooMany(n)
let Some(chat) = prefs.chat else fail NoChat
```

- **Gain:** says what it means. It is a word, not a symbol (principle 4).
- **Cost:** a second way to write `return Err(…)`. `eye fmt` should then rewrite one into the
  other, or the language gains two forms again.
- **Verdict:** worth it only with that formatter rule. Decide after 5.2 and 5.3 land and the
  lab has been rewritten with them.

### 5.5 Checks attached to the binding

The owner asked why `t == ""` can't fold into the first line. A trailing clause on `let` does it:

```text
let t = text.trim ensure t != "" else Empty
let n = parseInt(t) ? NotANumber(t) ensure n <= 99 else TooMany(n)
```

- **Meaning.** `let x = e ensure c else y` binds `x`; if `c` is false, the function fails with
  `y`, as `? y` would. A standalone `ensure c else y` statement covers checks with no binding.
- **Gain:** it is the shortest form, and it puts each value's conditions on its own line.
- **Costs:**
  - a new keyword (contextual);
  - a name used in its own `let`'s trailing clause;
  - a third place where `else` means "otherwise fail".
- **Rejected alternatives:**
  - `where`, which already means trait bounds;
  - `?` on a `Bool` (`n <= 99 ? TooMany(n)`), which is too cryptic;
  - Swift's comma-separated conditions (`let t = …, t != "" else …`), which can't give each
    condition its own error.
- **Verdict:** promising, but it should be decided on evidence. Rewrite the lab with 5.2 and 5.3
  first, then count how many guards still sit right after the `let` they check.

### 5.6 Pipes

The owner asked whether pipes should be more ergonomic, or change in scope. They liked option
P2.

**P1. `.` is the call; `()` only with arguments.** After a dot, a name that isn't a field calls
the function:

```text
let s = text.trim.lower.split(",")
var dirs = path.split("/").reverse.drop(1).reverse
```

- **Safe:** §8 already forbids a record type from having a method named like a field that holds
  a function, so `x.name` can't mean two things.
- **Cost:** a computed value now looks like a stored field. Kotlin, Swift, Ruby and Scala accept
  this.
- **Gain:** the 251 argument-free pipe stages and the `.size()` calls get one shorter spelling.

**P2. `|>` feeds a value into an expression, not only into a call.**

```text
text.trim |> match {
  "" => Err(Empty)
  t => …
}
n |> Ok                            # a function value: Ok(n)
total |> format(_, unit: "bytes")  # _ places the value, as today
x |> _ * 2                         # any expression that uses _
```

- **Rules:**
  - The right side is a call, which takes the value first (as today), unless it uses `_`.
  - Or it is a function value, which is applied.
  - Or it is any expression that uses `_`.
  - Or it is `match { … }` with no subject, which matches the value.
- **Effect:** existing pipes keep their meaning. `|>` gets the one job `.` can't do: feeding a
  value into something that isn't a method of it.

**P3. One form per call, kept by `eye fmt`.** With P1 and P2, `eye fmt` rewrites `x |> f(a)` into
`x.f(a)` and `x |> f` into `x.f`, and keeps `|>` where the right side isn't such a call. This
settles G.15's "write whichever reads better" the way principles 4 and 10 ask. One case
keeps `|>`. Inside an entity, `shapes |> add(s)` calls the collection function, not the entity's
`add` (§14), so the formatter leaves a pipe alone when the entity has a method of that name.

**P4. A form for updating a variable.** 24 real lines read `x = x |> f(…)`, and the ergonomics
review's commonest fight was `counts = counts |> insert(w, (counts[w] ?? 0) + 1)`. Options:
- `counts |>= insert(w, …)`, by analogy with `+=`;
- `counts[w] = …` for maps, which the ergonomics review proposed for its message 15.

The second covers the commonest case without a new operator.

**P5. Precedence of `??` and `|>`.** Four lines write `(html.slice(a..b) ?? "") |> lower`, because
`??` is the loosest operator. With P1 most of these become `html.slice(a..b) ?? ""` followed by a
method, and the parentheses stay for the rest. Changing the precedence would break the 20 lines
with `?? default` after a pipe. **Leave it.**

**P6. Field lambdas.** `.name` alone as a function, as in Swift's key paths:
`users.map(.name)`, `pins.filter(.active)`. This would remove 43 of the 71 field lambdas; the 28
comparisons stay lambdas. This one is the most separable from the rest, and can wait.

### 5.7 Calls without parentheses: `task.send "joko", 24 h`

The owner asked whether a call can drop its parentheses when it has arguments, as in Ruby,
Elixir and Nim. Today `log.info "done"` is `expected a line break or ';'`. That makes the form
free to add at the start of a statement, and only there. Elsewhere a name followed by a value
already has a meaning, or would collide with one:

| Written | What it could mean | Conflict |
|---|---|---|
| `f [x]` | `f([x])`, or `f[x]`, an index | index syntax |
| `f -x` | `f(-x)`, or `f - x` | binary minus |
| `f (a, b)` | two arguments, or one tuple | tuple syntax |
| `if ready { … }` | a block, or `ready({ … })` with a record | §4's rule that every `{` has one reading |
| `g(f a, b)` | `g(f(a), b)`, or `g(f(a, b))` | nesting |
| `let ok = check a == b` | `check(a == b)`, or `check(a) == b` | precedence |
| `24 h` | a `Duration` literal (§23: `number unit`) | juxtaposition already means "number, then unit" |

A narrow rule avoids every row: **a statement may start with a call without parentheses, and its
arguments run to the end of the line.**

```text
log.info "done"
task.send "joko", 24 h
notify.send! chat, "the build is green"
```

Inside an expression the parentheses stay: on the right of `=`, as an argument, or in a
condition. An argument that is itself a call keeps them: `log.info format(x)`.

- **Gain:** the commonest statements, logging, sending and recorded steps, lose two characters
  and read like commands.
- **Costs:**
  - two forms of every statement call, so `eye fmt` must pick one;
  - a call reads differently as a statement and as a value;
  - an argument that starts with `[`, `(` or `-` needs its own rule, or parentheses.
- **Earlier decision.** In the spec's first design session, the owner said "don't do bracing now it can be changed
  later".
- **Verdict:** possible, but it is a change of look rather than of what can be said. Decide after
  P1 (5.6), which removes the commonest `()` (calls without arguments) with none of these
  conflicts.

### 5.8 A dot for errors: `.NewErr`

The owner has seen languages where a leading dot names a case, and asked whether `.NewErr` could
mean an error. Two languages do something like it:
- **Swift:** `.failure(.empty)` names a case of the type the context expects.
- **Zig:** `return error.NotFound`, and `.foo` for an enum literal whose type is inferred.

(Erlang ends a clause with `.`, but that is a terminator, not a return value.)

In Eyelang this runs into three facts:

1. **Variants are already bare names.** It is `Empty`, not `ParseError.Empty`, so a dot to name
   a case saves nothing. The only thing it could add is the meaning "fail with this", which
   makes `.Empty` short for `return Err(Empty)`.
2. **A line that starts with `.` continues the line before** (§4), so a method chain can break
   before each `.`. A statement `.Empty` on a line of its own would become a method call on the
   previous line. It could only follow `then`, `else` or `?`.
3. **Principle 4 prefers words to symbols** unless the symbol is already universal, and a leading
   dot meaning "fail" isn't.

The same brevity comes from the forms above, which read as words:

```text
if t == "" then .Empty                  # the dot form
if t == "" then fail Empty              # 5.4
let n = parseInt(t) ? NotANumber(t)     # 5.2
let Some(c) = prefs.chat else fail NoChat
```

A leading dot is better spent on P6, field lambdas (`users.map(.name)`): only one meaning can
have it. **Verdict: no.** Take the idea's real point, one short marker for "fail", as `? x` and
`fail x`.

## 6. Recommendation and plan

**Phase 0, now, with no syntax change** (allowed under the checkpoint, since it only changes docs):
1. Rewrite `parseQuantity` as in section 3, and fix `errors.md` on `mapErr` (5.1).
2. Make §23 of SPECS say that `return`, `break` and `continue` are expressions, as `grammar.js`
   does, and add `then` to §4's contextual keywords.
3. Run `eye test` on the tutorial, `go test ./docs/...` and `TestDocsPrograms`.

**Phase 1, owner decisions, spec text only:**
1. Open question 13: the `then` form for one-line `if`s, with `eye fmt` rewriting.
2. A new open question: one form per call (P1 and P3), replacing G.15's "write whichever reads
   better".
3. A new open question: the failure path (5.2, 5.3, and later 5.4 and 5.5).

**Phase 2, after v0.4 or whenever the owner says, one change at a time.** Each follows CLAUDE.md:
- `grammar.js`, `syntax/`, the corpus, valid and invalid `testdata/syntax`;
- the checker and lowering (each is sugar over constructs that exist, so `interp` and `cgen`
  should not change);
- an `eye fix` or `eye fmt` rewrite of existing code;
- every example in SPECS and the docs.

| Order | Change | Size | Why in this order |
|---|---|---|---|
| 1 | `let … else` without braces (5.3) | small | consistent with the Blocks rule; nothing else depends on it |
| 2 | `eye fmt`: one form of `if` (OQ13) | small | removes the mixed guards before new forms arrive |
| 3 | `?` with an argument (5.2) | medium | the largest gain; fills the `Option`-to-`Result` gap |
| 4 | `()` only with arguments (P1) | medium | needed before P3 can rewrite pipes |
| 5 | `\|>` into expressions, and `match` without a subject (P2) | medium | gives `\|>` its own job |
| 6 | `eye fmt`: one form per call (P3) | small, but touches every file | only after 4 and 5 |
| 7 | `counts[w] = …` (P4) | medium | separate; the ergonomics review already wants it |
| 8 | `fail`, `ensure`, `.field` (5.4, 5.5, P6) | each small to medium | decide on the lab rewritten with 1 to 6 |
| 9 | statement calls without parentheses (5.7) | medium | a change of look; decide last |

After steps 1 to 6, `parseQuantity` reads:

```text
fn parseQuantity(text: String) -> Result(Int, ParseError) {
  let t = text.trim
  if t == "" then return Err(Empty)
  let n = parseInt(t) ? NotANumber(t)
  if n > 99 then Err(TooMany(n)) else Ok(n)
}
```

If 5.4 and 5.5 are accepted later as well, it reads:

```text
fn parseQuantity(text: String) -> Result(Int, ParseError) {
  let t = text.trim ensure t != "" else Empty
  let n = parseInt(t) ? NotANumber(t) ensure n <= 99 else TooMany(n)
  Ok(n)
}
```

## 7. Coordination, 2026-10-13

- **Main session (eyelang-f4).** It is working on the C backend (7.13), hosted mode (7.6) and
  checker messages. Nothing in flight touches the parser, the grammar, or SPECS §6, §10 or §23.
  It asked for this to go to the owner as a proposal, timed against the native-focus checkpoint.
- **7.6 session.** It works in `cgen/` and `lir/`, and isn't editing the tutorial or
  `docs/future-of-eyelang.md`.
- **Web editor session.** It made the tutorial programs the playground's examples (6ae5300). Phase
  0 must keep `TestDocsPrograms` and the playground's end-to-end test green.
- **Already decided elsewhere:**
  - `let … else` exists, and its `else` must leave (#120).
  - G.15 (functions as methods, pipes stay).
  - G.17 (`return` stays out of lambdas).
  - Open question 14, `if let`: the ergonomics review would keep `match`. Nothing here changes
    that. `if let Some(a) = answer then return Err(Taken(a))` fails when a value is present,
    the opposite of what `?` and `let … else` handle.
