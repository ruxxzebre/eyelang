---
title: Tutorial
---

# Tutorial

This tutorial teaches Eyelang by writing small programs, from plain values up to a job that waits
fifteen minutes and survives a restart. It assumes you can program in some other language, and
that you have `eye` installed ([Quickstart](/quickstart)).

## What you'll build

The examples follow a small shop:

1. [Values and functions](/tutorial/values-and-functions): prices and a shopping basket, with
   `let`, functions, lists, pipes and your first tests.
2. [Types and matching](/tutorial/types-and-matching): a to-do list, with records, variant types,
   exhaustive `match`, `Option` and methods.
3. [Errors](/tutorial/errors): reading an order and taking it from the stock, with `Result`, `?`
   and `mut`.
4. [An entity](/tutorial/an-entity): the shop's shelves as entities, which own their state and
   change it in transactions.
5. [A job that waits](/tutorial/a-job-that-waits): a hold on stock that ends after fifteen
   minutes, unless the order is paid for first.
6. [All together: approvals](/tutorial/approvals): a larger program that uses everything above.

Each page takes five to ten minutes to read and builds on the one before it.

## How the pages work

Each page has one program behind it, in
[`docs/tutorial/code/`](https://github.com/ruxxzebre/eyelang/tree/dev/docs/tutorial/code), one
directory per page. The code you see on a page is taken straight from that program, and its `test`
blocks check everything the page says. The test suite runs every one of them, so the examples
always work with the current `eye`.

To follow along, run a page's program yourself from the repository's root:

```sh
eye test docs/tutorial/code/values-and-functions
```

Or copy a page's code into a directory of your own with an empty `eye.toml`, and change it.

Each page ends with links to the [language reference](/reference/language/), which states the
full rules. The tutorial shows the common cases and leaves the rest to it.
