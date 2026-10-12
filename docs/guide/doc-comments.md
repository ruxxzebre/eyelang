---
title: Doc comments
---

# Doc comments

A comment that starts with `##` documents the code directly below it. The same text serves three
readers: `eye doc` publishes it in the API reference, your editor shows it on hover and in
completion lists, and a model call uses it to describe that part of a schema. Write it once, for all
three.

A comment that starts with a single `#` is a note for whoever maintains the code. It is never
published.

## What a doc comment documents

A `##` block documents the declaration, member, field or variant right below it:

```eye
## Train reminders: when to leave for the next train home.
```

<<< ./code/doc-comments/trains.eye#types

<<< ./code/doc-comments/trains.eye#next

- **Declarations:** types, functions, entities, jobs, views and contracts.
- **Members:** a field of a record, a variant (the comment goes above its `|`), an entity's `state`
  and its methods.
- **The module:** a `##` block at the top of a file, with a blank line between it and the first
  declaration. Here that's "Train reminders: when to leave for the next train home."

A blank line or a plain `#` comment between a doc comment and the code detaches it, so what a doc
comment documents is always the code it touches.

The code on this page is a [real program](https://github.com/ruxxzebre/eyelang/tree/dev/docs/guide/code/doc-comments)
that the test suite checks and runs.

## Write the first sentence as the summary

Doc comments are Markdown. Their first sentence is the summary: the API reference's index tables,
completion lists and hover headers show only that sentence. Make it stand on its own and say what
the thing is or does:

| Instead of | Write |
|---|---|
| `## This function is used to compute the next train.` | `## The first departure that leaves after now, if there is one.` |
| `## Helper.` | `## Rounds a delay up to whole minutes.` |
| `## See below.` | Put the important part first. |

After the first sentence, say what a caller needs to know: what happens at the edges, what an error
means, and when to use something else instead. Use backticks for code, and a fenced `eye` block for
an example.

## Link with brackets

`[Name]` links to a declaration by name, wherever it is documented:

| You write | It links to |
|---|---|
| `[Departure]` | a declaration in the same module, the prelude or the built-in functions |
| `[Departure.delay]` | a field, variant, state or method of a declaration |
| `[Some]` | a variant, by its own name |
| `[ai.generate]` | a declaration in another module the program uses |

`eye doc` checks every link. If one names nothing, for example after a rename, it reports where and
exits with status 1, so CI catches docs that point at code that's gone:

```text
main.md: the doc comment of next links to [Departure.late], which names nothing documented
```

## See your docs

Run `eye doc` on your program to write its API reference as Markdown, next to the standard library's:

```sh
eye doc --out api .
```

`eye doc` replaces the folder it writes to, but only one that is missing, empty or full of pages it
wrote before. If the folder holds anything else, such as code, it stops without changing anything
and asks you to pass `--out` with another folder.

This site's [API reference](/reference/api/) is the standard library's, written the same way from
the `##` comments in its source. See [the eye command](/reference/cli#doc) for the options.

## Docs that models read

In a model call, the type you ask for becomes a JSON Schema, and its doc comments become the
schema's descriptions. A field's `##` comment tells the model what to put there. So the doc comment
that helps a person read your code also helps the model fill the type in correctly. See
[AI models and agents](/reference/language/ai-models-and-agents).
