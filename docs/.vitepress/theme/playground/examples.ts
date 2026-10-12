// The playground's examples are the programs the quickstart, the tutorial
// and the guide are written from, so each runs in the playground as the docs
// show it, and those of docs/examples, written for the playground and the
// examples page. The Go test internal/playground TestDocsPrograms checks
// that each passes its tests.

import type { Files } from './engine'

// args are the arguments Run passes to main, filled in when the example is
// chosen, for an example whose main needs some.
export type Example = { id: string; group: string; title: string; files: Files; args?: string }

const sources = import.meta.glob(
  ['../../../quickstart/code/**/*', '../../../tutorial/code/**/*', '../../../guide/code/**/*', '../../../examples/**/*'],
  { query: '?raw', import: 'default', eager: true },
) as Record<string, string>

// The menu's groups, in the order the menu shows them.
const groups: Record<string, string> = { quickstart: 'Start', examples: 'Examples', tutorial: 'Tutorial', guide: 'Guide' }

// The order and titles of the examples. An example missing here goes last
// in its group, under its directory's name.
const titles: Record<string, string> = {
  'quickstart/remind': 'Quickstart: a reminder',
  'examples/trial-ending': 'Trial ending: an email three days before',
  'examples/webhook-delivery': 'Webhook delivery: keys, lost replies and backoff',
  'examples/uptime-monitor': 'Uptime monitor: probe every minute, alert once',
  'examples/daily-report': 'Daily report: 08:00 in New York, every weekday',
  'examples/refund-approval': 'Refund approval: a person says yes first',
  'examples/log-summary': 'Log summary: the plain language',
  'examples/shop-and-bank': 'A shop and its bank: two programs, one contract',
  'tutorial/hello': 'Hello',
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

// What Run passes to main, for the examples whose main needs arguments.
const args: Record<string, string> = {
  'quickstart/remind': 'inv-1 ann@example.com',
  'examples/trial-ending': 'ann ann@example.com',
}

function collect(): Example[] {
  const byId = new Map<string, Example>()
  for (const [path, text] of Object.entries(sources)) {
    const m = /\/(?:(quickstart|tutorial|guide)\/code|(examples))\/([^/]+)\/(.+)$/.exec(path)
    if (!m) continue
    const [, docs, examples, name, file] = m
    const group = docs ?? examples
    const id = `${group}/${name}`
    let ex = byId.get(id)
    if (!ex) {
      ex = { id, group: groups[group], title: titles[id] ?? name, files: {}, args: args[id] }
      byId.set(id, ex)
    }
    ex.files[file] = text
  }
  const groupOrder = Object.values(groups)
  const order = Object.keys(titles)
  const rank = (id: string) => (order.includes(id) ? order.indexOf(id) : order.length)
  return [...byId.values()].sort(
    (a, b) => groupOrder.indexOf(a.group) - groupOrder.indexOf(b.group) || rank(a.id) - rank(b.id) || a.id.localeCompare(b.id),
  )
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

// The example the playground opens with.
export const defaultExample = 'quickstart/remind'

export function example(id: string): Example | undefined {
  return id === blank.id ? blank : examples.find((e) => e.id === id)
}
