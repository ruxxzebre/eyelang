# What SPECS.md §12's first example assumes (context=llm): the items to
# summarise, a prompt made of them, and the model service.

import llm

type Item = { title: String, url: String }

fn prompt(items: List(Item)) -> String = items |> map(fn i: "- {i.title}") |> join("\n")
