// The playground's examples are the programs the tutorial and the guide are
// written from, so each runs in the playground as the docs show it, and the
// Go test internal/playground TestDocsPrograms checks that it does.

import type { Files } from './engine'

export type Example = { id: string; group: string; title: string; files: Files }

const sources = import.meta.glob(['../../../tutorial/code/**/*', '../../../guide/code/**/*'], {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

// The order and titles of the tutorial's and guide's pages.
const titles: Record<string, string> = {
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
    const m = /\/(tutorial|guide)\/code\/([^/]+)\/(.+)$/.exec(path)
    if (!m) continue
    const [, group, name, file] = m
    const id = `${group}/${name}`
    let ex = byId.get(id)
    if (!ex) {
      ex = { id, group: group === 'tutorial' ? 'Tutorial' : 'Guide', title: titles[id] ?? name, files: {} }
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
