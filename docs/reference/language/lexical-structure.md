---
title: "Lexical structure"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Lexical structure

Source files are UTF-8. Names and keywords are ASCII; strings and comments may contain any text.

- **Comments:** `#` to the end of the line. A comment that starts with `##` is a doc comment: it
  documents the declaration, member, field or variant directly below it. A blank line or a plain
  comment in between detaches it. `eye doc` publishes doc comments, editors show them on hover and
  in completion, and a model call uses them to describe that part of the schema ([section 19](/reference/language/ai-models-and-agents)).
  - A doc comment is Markdown. Its first sentence is the summary that indexes, completion lists
    and hover headers show, so it should say what the thing is or does on its own.
  - `[Name]`, `[Type.member]` and `[module.Name]` link to the declaration they name. `eye doc`
    turns them into links and fails on a link that names nothing, so docs can't point at code that
    was renamed or removed.
  - A `##` block at the top of a file, separated from the first declaration by a blank line,
    documents the module. Plain `#` comments are never published: use them for notes to whoever
    maintains the code.
- **Names:** a letter or `_`, then letters, digits and `_`. A name that starts with an upper-case
  letter names a type, variant, entity or contract (`World`, `Some`, `Notify`); any other name is a
  value, function, field or module (`world`, `markAsleep`, `zeropod`). Names can no longer end in
  `!` or `?` (Eyelang 1 allowed `update!` and `ok?`): `!` now marks waiting calls and `?`
  propagates errors.
- **Literals:**

  | Kind | Examples |
  |---|---|
  | Int (arbitrary precision) | `0`, `42`, `1_000_000`, `0xC2`, `0b1010_0101` |
  | Float (64-bit IEEE) | `1.5`, `0.25`, `6.02e23` |
  | String, with interpolation | `"hello"`, `"ireland-{day}"`, `"\{literal brace}"` |
  | Bool | `true`, `false` |
  | Char | `'a'`, `'\n'`, `'\u{1F600}'` |
  | Unit | `()` |
  | Duration | `500 ms`, `20 min`, `3 h`, `2 days`, `1.5 h` |
  | Size | `512 B`, `64 KiB`, `64 MiB`, `6 GiB` |
  | Time of day | `07:40`, `23:59:30` |
  | Date | `2026-10-10` |
  | Instant | `2026-10-10 07:40 in "Europe/Dublin"`, `2026-10-10 07:40 in dublin` |
  | Next time of day in a zone | `07:45 in "Europe/Dublin"`, `07:45 in dublin` |

  A number followed by a unit name (`ms`, `s`, `min`, `h`, `day`, `days`, `week`, `weeks`, `B`,
  `KiB`, `MiB`, `GiB`) is a Duration or Size. A date or time of day is written without spaces;
  `2026 - 10 - 10` is still subtraction. Times are two digits each: `7:40` is not a time. A
  literal names a real date or time, or is an error at it: hours go from `00` to `23`, minutes
  and seconds from `00` to `59`, months from `01` to `12`, and the day must exist in its month,
  so `25:00`, `12:60` and `2026-02-30` are errors. Midnight is `00:00`; `24:00` is not a time. The zone
  after `in` is a zone's name, or a name for a `Zone` value: `dublin` after
  `pub let dublin: Zone = "Europe/Dublin"`.

  An integer may be written in hexadecimal after `0x` (`0xC2`, `0xFFFF_FFFF`) or in binary after
  `0b` (`0b1010_0101`), with `_` between digits as in decimal, and is an `Int`, or any
  fixed-width type where one is expected: `let lead: Byte = 0xC2`. The prefix is lower case; hex
  digits may be either, and `eye fmt` writes them upper case, keeping the base the author wrote,
  so a byte range stays `0xC2..=0xDF` and a mask stays binary. A leading `0` doesn't make a
  number octal: `007` is 7. `0x` with no hex digit after it is an error that says so.
- **Characters:** `'a'` is a `Char`, one Unicode scalar value, with the escapes of strings plus
  `\'`. Where a `Byte` is expected, an ASCII character literal is a `Byte`: `src[i] == Some('"')`
  ([section 5](/reference/language/values-and-types)).
- **Strings:** a string's text stays on one line, though an interpolated expression may break
  across lines. The escapes are `\n`, `\r`, `\t`, `\0`, `\\`, `\"`, `\{`, `\}` and
  `\u{1F600}`. `{expression}` interpolates a value that has `Show`.

  A **multi-line string** starts with `"""` at the end of a line and ends at a `"""` with only
  spaces before it on its line. Those spaces are the margin: every line starts with it, or is
  blank, and loses it, and the line breaks after the opening `"""` and before the closing line
  aren't part of the string. One or two quotes inside are text; escapes and interpolation work as in other
  strings.

  ```eye
  let prompt = """
    You write a morning briefing for {reader}.
      - this line keeps two spaces
    Say "Nothing new." when nothing matters.
    """
  ```
- **Keywords.** These are reserved and are never names: `let var mut fn pub return if else match
  for in while break continue true false _`. Every other keyword is a keyword only where the grammar
  expects it, and an ordinary name everywhere else, so `state`, `view`, `type` and `test` work as
  variable and field names:
  - at the start of a declaration: `import type opaque trait impl entity job view contract test
    extern`;
  - inside declarations: `state` and `migrate … from` in entities; `command`, `query`, `event` and
    `on` in contracts; `uses` and `where` after a signature;
  - `with` at the start of a statement and inside a record update, `dyn` at the start of a type,
    and the unit names after a number.

  A contextual keyword is read as a keyword wherever one is allowed, so a variable named `with`
  can't begin a statement. `in` and `for` may also be argument labels: `daily(07:40, in: zone)`.
- **Attributes:** `@name` or `@name(args)` before a declaration or member.
- **One style.** `eye fmt` keeps the line breaks you chose and fixes everything else: two spaces
  of indentation, spacing around operators and after commas and colons, `;` turned into line
  breaks, trailing commas in lists that end on their own line, aligned trailing comments, and
  hex digits in upper case (`0xc2` is written `0xC2`). It has no options. Formatting a program's directory also writes public functions' `uses` clauses
  ([section 12](/reference/language/effects-and-signatures)).
- **Newlines end statements.** A line break ends a statement, declaration, member or match arm
  wherever one could end. It doesn't when the next line (ignoring blank and comment lines) starts
  with a binary operator, `.`, `?`, `,`, `:`, `=`, `->` or `else`; `-` and `!` continue only as
  `->` and `!=`, since `-x` and `!x` can start a statement. Inside `(…)`, `[…]` and record or map
  braces no statement can end, so line breaks there are ordinary spaces (until a lambda's block
  starts a new list of statements). `;` separates statements on one line.

  ```eye
  let ready = worlds
    |> filter(fn w: w.phase == Running)      # continues: the line starts with |>
    |> size
  let next = a
  (b)                                        # a new statement, not the call a(b)
  ```
- **Braces.** Braces hold blocks of statements and data; layout is free and indentation means
  nothing. A branch that is one expression needs no braces: `if n < 2 then n else fib(n - 1)`,
  `Some(v) => v`, `fn x: x + 1`, `fn double(n) = n * 2`.

  `{` opens a block after `if …` (without `then`), `else` (after a block), `for … in …`, `while`,
  `let … else`, a function or job signature, `test "…"` and a lambda's parameters. In any other
  expression position it starts a record (`{ name: "Ann" }`), a map (`{ "a": 1 }`), an update
  (`{ user with age: 4 }`) or the empty `{}`. After a match arm's `=>`, `{` opens a block unless
  `name:`, `name,` or `name with` follows, since only a record starts that way:
  `Some(u) => { u with seen: true }` returns a record, `None => {}` is an empty block, and a record
  with one shorthand field, `{ u }`, or a map needs parentheses there: `=> ({ u })`.
- **Waiting calls:** in `f!(x)` the `!` and `(` follow the callee with no spaces. `x != y` and `!x`
  are the usual operators.
