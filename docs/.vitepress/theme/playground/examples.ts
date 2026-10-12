// The playground's examples are the programs the tutorial and the guide are
// written from, so each runs in the playground as the docs show it, and those
// of docs/examples, written for the playground. The Go test
// internal/playground TestDocsPrograms checks that each passes its tests.

import type { Files } from './engine'

export type Example = { id: string; group: string; title: string; files: Files }

const sources = import.meta.glob(['../../../tutorial/code/**/*', '../../../guide/code/**/*', '../../../examples/**/*'], {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

const groups: Record<string, string> = { tutorial: 'Tutorial', guide: 'Guide', examples: 'Programs together' }

// The order and titles of the examples, the tutorial's and the guide's pages.
const titles: Record<string, string> = {
  'examples/shop-and-bank': 'A shop and its bank: a contract between two programs',
  'tutorial/hello': 'Hello (quickstart)',
  'tutorial/values-and-functions': '1. Values and functions',
  'tutorial/types-and-matching': '2. Types and matching',
  'tutorial/errors': '3. Errors',
  'tutorial/an-entity': '4. An entity',
  'tutorial/a-job-that-waits': '5. A job that waits',
  'tutorial/approvals': 'All together: approvals',
  'guide/entities': 'Entities',
  'guide/jobs-and-waiting': 'Jobs and waiting',
  'guide/views-events-schedules': 'Views, events and schedules',
  'guide/testing': 'Testing with time and recordings',
  'guide/contracts': 'Talking to other services',
  'guide/effects': 'Effects',
  'guide/ai-calls': 'AI calls',
  'guide/doc-comments': 'Doc comments',
}

function collect(): Example[] {
  const byId = new Map<string, Example>()
  for (const [path, text] of Object.entries(sources)) {
    const m = /\/(?:(tutorial|guide)\/code|(examples))\/([^/]+)\/(.+)$/.exec(path)
    if (!m) continue
    const [, docs, examples, name, file] = m
    const group = docs ?? examples
    const id = `${group}/${name}`
    let ex = byId.get(id)
    if (!ex) {
      ex = { id, group: groups[group], title: titles[id] ?? name, files: {} }
      byId.set(id, ex)
    }
    ex.files[file] = text
  }
  const order = Object.keys(titles)
  const rank = (id: string) => (order.includes(id) ? order.indexOf(id) : order.length)
  return [...byId.values()].sort((a, b) => rank(a.id) - rank(b.id) || a.id.localeCompare(b.id))
}

export const examples = collect()

export const blank: Example = {
  id: 'blank',
  group: '',
  title: 'Empty program',
  files: {
    'eye.toml': '',
    'main.eye': 'pub fn main(args: List(String)) uses {stdio} {\n  print("Hello!")\n}\n',
  },
}

export function example(id: string): Example | undefined {
  return id === blank.id ? blank : examples.find((e) => e.id === id)
}
