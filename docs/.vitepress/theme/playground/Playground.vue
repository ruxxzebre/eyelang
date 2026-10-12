<script setup lang="ts">
// The online playground (SPECS.md, open question 34): write Eyelang, and
// check, run, test and format it in the browser, with the editor support
// `eye lsp` gives. Nothing leaves the page: the engine is eye's own Go,
// compiled to WebAssembly, in a worker.

import { EditorView } from '@codemirror/view'
import type { EditorState } from '@codemirror/state'
import { withBase } from 'vitepress'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, shallowRef, watch } from 'vue'
import { createState, toOffset, type Hooks } from './editor'
import { Engine, type Diagnostic, type Files, type Location, type Status } from './engine'
import { blank, example, examples } from './examples'
import { encode, fromHash, load, save } from './share'

type Line = { kind: 'stdout' | 'stderr' | 'info' | 'pass' | 'fail' | 'error'; text: string }

const host = ref<HTMLElement>()
const engine = shallowRef<Engine>()
const view = shallowRef<EditorView>()
const states = new Map<string, EditorState>()

const files = reactive<Files>({})
const active = ref('main.eye')
const args = ref('')
const exampleId = ref('')
const status = ref<Status>({ state: 'loading' })
const output = ref<Line[]>([])
const problems = ref<Diagnostic[]>([])
const panel = ref<'output' | 'problems'>('output')
const busy = ref<'' | 'run' | 'test'>('')
const notice = ref('')

// Tabs: main.eye first, then the other source files, then the rest.
const paths = computed(() =>
  Object.keys(files).sort((a, b) => {
    const rank = (p: string) => (p === 'main.eye' ? 0 : p.endsWith('.eye') ? 1 : p === 'eye.toml' ? 2 : 3)
    return rank(a) - rank(b) || a.localeCompare(b)
  }),
)

const groups = computed(() => {
  const out: Record<string, typeof examples> = {}
  for (const e of examples) (out[e.group] ??= []).push(e)
  return out
})

const loadingText = computed(() => {
  const s = status.value
  if (s.state === 'failed') return `The engine didn't start: ${s.error}`
  if (s.state === 'ready') return ''
  const mb = (n = 0) => (n / 1e6).toFixed(1)
  return s.loaded ? `Loading the engine… ${mb(s.loaded)}${s.total ? ` of ${mb(s.total)}` : ''} MB` : 'Loading the engine…'
})

const hooks: Hooks = {
  onChange(path, text) {
    files[path] = text
    engine.value?.setFiles(files)
    persist()
  },
  onRun: () => run(),
  onTest: () => test(),
  onFormat: () => format(),
  onJump: (loc) => jump(loc),
  onProblems(all) {
    problems.value = all
  },
}

function stateFor(path: string): EditorState {
  let s = states.get(path)
  if (!s) {
    s = createState(engine.value!, path, files[path] ?? '', hooks)
    states.set(path, s)
  }
  return s
}

function open(path: string) {
  if (!view.value || !(path in files)) return
  states.set(active.value, view.value.state)
  active.value = path
  view.value.setState(stateFor(path))
  view.value.focus()
  persist()
}

function useFiles(next: Files, first?: string) {
  for (const k of Object.keys(files)) delete files[k]
  Object.assign(files, next)
  states.clear()
  engine.value?.setFiles(files)
  problems.value = []
  const start = first && first in next ? first : 'main.eye' in next ? 'main.eye' : paths.value[0]
  active.value = start ?? 'main.eye'
  if (view.value && start) view.value.setState(stateFor(start))
  persist()
}

function chooseExample(id: string) {
  const e = example(id)
  if (!e) return
  exampleId.value = id
  useFiles(e.files)
  output.value = [{ kind: 'info', text: `${e.title}: Run (Ctrl+Enter) runs main; Test (Ctrl+Shift+Enter) runs the tests.\n` }]
  if (location.hash) history.replaceState(null, '', location.pathname + location.search)
}

let saveTimer: ReturnType<typeof setTimeout> | undefined
function persist() {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => save({ files: { ...files }, active: active.value, args: args.value, example: exampleId.value }), 300)
}
watch(args, persist)

function jump(loc: Location) {
  if (loc.path !== active.value) open(loc.path)
  const v = view.value
  if (!v) return
  const from = toOffset(v.state.doc, loc.range.start)
  const to = toOffset(v.state.doc, loc.range.end)
  v.dispatch({ selection: { anchor: from, head: to }, effects: EditorView.scrollIntoView(from, { y: 'center' }) })
  v.focus()
}

function print(kind: Line['kind'], text: string) {
  const last = output.value[output.value.length - 1]
  if (last && last.kind === kind) last.text += text
  else output.value.push({ kind, text })
  nextTick(() => {
    const el = document.querySelector('.pg-output')
    if (el) el.scrollTop = el.scrollHeight
  })
}

function splitArgs(s: string): string[] {
  const out: string[] = []
  for (const m of s.matchAll(/"([^"]*)"|'([^']*)'|(\S+)/g)) out.push(m[1] ?? m[2] ?? m[3])
  return out
}

async function run() {
  if (!engine.value || busy.value) return
  busy.value = 'run'
  panel.value = 'output'
  output.value = []
  const started = performance.now()
  try {
    const rep = await engine.value.run(splitArgs(args.value), (stream, text) => print(stream === 'stdout' ? 'stdout' : 'stderr', text))
    if (rep.diagnostics.length) problems.value = rep.diagnostics
    const ms = Math.round(performance.now() - started)
    print('info', `\n${rep.exit === 0 ? 'exit 0' : `exit ${rep.exit}`} · ${ms} ms\n`)
  } catch (err: any) {
    print('error', `\n${err.message}\n`)
  } finally {
    busy.value = ''
  }
}

async function test() {
  if (!engine.value || busy.value) return
  busy.value = 'test'
  panel.value = 'output'
  output.value = []
  try {
    const rep = await engine.value.test('')
    if (rep.diagnostics.length) {
      problems.value = rep.diagnostics
      print('stderr', rep.output)
      return
    }
    for (const line of rep.output.split(/(?<=\n)/)) {
      const kind = line.startsWith('ok ') ? 'pass' : line.startsWith('FAIL') ? 'fail' : line.startsWith('      ') ? 'stdout' : 'info'
      print(kind, line)
    }
  } catch (err: any) {
    print('error', `\n${err.message}\n`)
  } finally {
    busy.value = ''
  }
}

function stop() {
  engine.value?.stop('stopped')
}

async function format() {
  const v = view.value
  if (!engine.value || !v || !active.value.endsWith('.eye')) return
  try {
    const { text } = await engine.value.format(active.value)
    if (text !== v.state.doc.toString()) {
      v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: text } })
    }
  } catch (err: any) {
    flash(err.message)
  }
}

function flash(text: string) {
  notice.value = text
  setTimeout(() => notice.value === text && (notice.value = ''), 3000)
}

async function share() {
  const code = await encode({ ...files })
  const url = `${location.origin}${location.pathname}#code=${code}`
  history.replaceState(null, '', url)
  try {
    await navigator.clipboard.writeText(url)
    flash('Link copied: it holds the whole program')
  } catch {
    flash('The link is in the address bar')
  }
}

const validName = (name: string) => /^([a-z][a-z0-9_]*\/)*[A-Za-z0-9_.-]+\.(eye|json|toml)$/.test(name)

function addFile() {
  const name = prompt('File name, such as notify/notify.eye or recordings/notify.json')?.trim()
  if (!name) return
  if (!validName(name)) return flash(`${name}: a file is a .eye, .json or .toml, in lower-case directories`)
  if (name in files) return open(name)
  files[name] = name.endsWith('.eye') ? '' : name.endsWith('.json') ? '{}\n' : ''
  engine.value?.setFiles(files)
  open(name)
}

function removeFile(path: string) {
  if (!confirm(`Delete ${path}?`)) return
  delete files[path]
  states.delete(path)
  engine.value?.setFiles(files)
  if (active.value === path) {
    const next = paths.value[0]
    if (next) {
      active.value = next
      view.value?.setState(stateFor(next))
    }
  }
  persist()
}

function renameFile(path: string) {
  const name = prompt('Rename to', path)?.trim()
  if (!name || name === path) return
  if (!validName(name)) return flash(`${name}: a file is a .eye, .json or .toml, in lower-case directories`)
  if (name in files) return flash(`${name} exists already`)
  if (view.value && active.value === path) states.set(path, view.value.state)
  files[name] = view.value && active.value === path ? view.value.state.doc.toString() : files[path]
  delete files[path]
  states.delete(path)
  engine.value?.setFiles(files)
  active.value = name
  view.value?.setState(stateFor(name))
  persist()
}

function problemAt(d: Diagnostic) {
  if (d.path) jump({ path: d.path, range: d.range })
}

const mac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)
const mod = mac ? '⌘' : 'Ctrl'

onMounted(async () => {
  const e = new Engine(withBase('/'))
  e.onStatus = (s) => (status.value = s)
  engine.value = e

  const fromLink = await fromHash(location.hash)
  const saved = load()
  let first: string | undefined
  if (fromLink?.files) {
    Object.assign(files, fromLink.files)
  } else if (fromLink?.example && example(fromLink.example)) {
    exampleId.value = fromLink.example
    Object.assign(files, example(fromLink.example)!.files)
  } else if (saved?.files && Object.keys(saved.files).length) {
    Object.assign(files, saved.files)
    first = saved.active
    args.value = saved.args ?? ''
    exampleId.value = saved.example ?? ''
  } else {
    exampleId.value = 'tutorial/hello'
    Object.assign(files, example('tutorial/hello')?.files ?? blank.files)
  }
  e.setFiles(files)
  active.value = first && first in files ? first : 'main.eye' in files ? 'main.eye' : paths.value[0]
  view.value = new EditorView({ state: stateFor(active.value), parent: host.value! })
  output.value = [{ kind: 'info', text: `Run (${mod}+Enter) runs main; Test (${mod}+Shift+Enter) runs the tests. Hover a name to see its type, ${mod}+click it to go to its definition.\n` }]
})

onBeforeUnmount(() => {
  view.value?.destroy()
  engine.value?.dispose()
})
</script>

<template>
  <div class="pg">
    <div class="pg-bar">
      <select class="pg-select" :value="exampleId" aria-label="Example" @change="chooseExample(($event.target as HTMLSelectElement).value)">
        <option value="" disabled>Examples…</option>
        <optgroup v-for="(list, group) in groups" :key="group" :label="group">
          <option v-for="ex in list" :key="ex.id" :value="ex.id">{{ ex.title }}</option>
        </optgroup>
        <option :value="blank.id">{{ blank.title }}</option>
      </select>
      <button class="pg-btn pg-primary" :disabled="status.state !== 'ready' || busy !== ''" :title="`Run main (${mod}+Enter)`" @click="run">
        {{ busy === 'run' ? 'Running…' : 'Run' }}
      </button>
      <button class="pg-btn" :disabled="status.state !== 'ready' || busy !== ''" :title="`Run the tests (${mod}+Shift+Enter)`" @click="test">
        {{ busy === 'test' ? 'Testing…' : 'Test' }}
      </button>
      <button v-if="busy" class="pg-btn" title="Stop the program" @click="stop">Stop</button>
      <button class="pg-btn" :disabled="status.state !== 'ready'" :title="`Format the file (${mod}+S)`" @click="format">Format</button>
      <button class="pg-btn" title="Copy a link that holds the whole program" @click="share">Share</button>
      <label class="pg-args">
        <span>Arguments</span>
        <input v-model="args" placeholder="Ann Bo" spellcheck="false" @keydown.enter="run" />
      </label>
      <span class="pg-status" :class="{ failed: status.state === 'failed' }" role="status">{{ notice || loadingText }}</span>
    </div>

    <div class="pg-main">
      <div class="pg-editor">
        <div class="pg-tabs" role="tablist">
          <div
            v-for="p in paths"
            :key="p"
            class="pg-tab"
            :class="{ active: p === active }"
            role="tab"
            :aria-selected="p === active"
            @click="open(p)"
            @dblclick="renameFile(p)"
          >
            <span>{{ p }}</span>
            <button v-if="paths.length > 1" class="pg-close" :aria-label="`Delete ${p}`" @click.stop="removeFile(p)">×</button>
          </div>
          <button class="pg-tab pg-add" title="Add a file" aria-label="Add a file" @click="addFile">+</button>
        </div>
        <div ref="host" class="pg-code" />
      </div>

      <div class="pg-side">
        <div class="pg-tabs" role="tablist">
          <button class="pg-tab" :class="{ active: panel === 'output' }" role="tab" @click="panel = 'output'">Output</button>
          <button class="pg-tab" :class="{ active: panel === 'problems' }" role="tab" @click="panel = 'problems'">
            Problems<span v-if="problems.length" class="pg-count">{{ problems.length }}</span>
          </button>
        </div>
        <pre v-show="panel === 'output'" class="pg-output" aria-live="polite"><span v-for="(l, i) in output" :key="i" :class="`pg-${l.kind}`">{{ l.text }}</span></pre>
        <ul v-show="panel === 'problems'" class="pg-problems">
          <li v-if="!problems.length" class="pg-none">No problems.</li>
          <li v-for="(d, i) in problems" :key="i" @click="problemAt(d)">
            <span v-if="d.path" class="pg-where">{{ d.path }}:{{ d.range.start.line + 1 }}:{{ d.range.start.character + 1 }}</span>
            {{ d.message }}
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<style>
.pg {
  --pg-k: #cf222e;
  --pg-c: #6e7781;
  --pg-s: #0a3069;
  --pg-n: #0550ae;
  --pg-t: #8250df;
  --pg-a: #953800;
  --pg-w: #116329;
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: calc(100vh - var(--vp-nav-height) - 48px);
  min-height: 480px;
  padding: 16px 24px;
}
@media (max-width: 640px) {
  .pg { padding: 12px 16px; }
}
.dark .pg {
  --pg-k: #ff7b72;
  --pg-c: #8b949e;
  --pg-s: #a5d6ff;
  --pg-n: #79c0ff;
  --pg-t: #d2a8ff;
  --pg-a: #ffa657;
  --pg-w: #7ee787;
}
.eye-k { color: var(--pg-k); }
.eye-c { color: var(--pg-c); font-style: italic; }
.eye-s { color: var(--pg-s); }
.eye-n { color: var(--pg-n); }
.eye-t { color: var(--pg-t); }
.eye-a { color: var(--pg-a); }
.eye-w { color: var(--pg-w); font-weight: 700; }

.pg-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.pg-select, .pg-args input {
  height: 32px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 6px;
  background: var(--vp-c-bg);
  color: var(--vp-c-text-1);
  padding: 0 8px;
  font-size: 14px;
}
.pg-select { max-width: 260px; }
.pg-btn {
  height: 32px;
  padding: 0 14px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 6px;
  background: var(--vp-c-bg-soft);
  color: var(--vp-c-text-1);
  font-size: 14px;
  font-weight: 500;
  transition: border-color 0.2s, background-color 0.2s;
}
.pg-btn:hover:not(:disabled) { border-color: var(--vp-c-brand-1); }
.pg-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.pg-primary { background: var(--vp-c-brand-3); border-color: var(--vp-c-brand-3); color: var(--vp-c-white); }
.pg-primary:hover:not(:disabled) { background: var(--vp-c-brand-2); }
.pg-args { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--vp-c-text-2); }
.pg-args input { width: 160px; font-family: var(--vp-font-family-mono); }
.pg-status { font-size: 13px; color: var(--vp-c-text-2); margin-left: auto; }
.pg-status.failed { color: var(--vp-c-danger-1); }

.pg-main { flex: 1; display: grid; grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); gap: 12px; min-height: 0; }
@media (max-width: 960px) {
  .pg { height: auto; }
  .pg-main { grid-template-columns: minmax(0, 1fr); grid-template-rows: 60vh 40vh; }
}
.pg-editor, .pg-side {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  overflow: hidden;
  background: var(--vp-c-bg);
}
.pg-code { flex: 1; min-height: 0; }
.pg-code .cm-editor { height: 100%; }
.pg-tabs { display: flex; overflow-x: auto; border-bottom: 1px solid var(--vp-c-divider); background: var(--vp-c-bg-soft); }
.pg-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  font-size: 13px;
  font-family: var(--vp-font-family-mono);
  color: var(--vp-c-text-2);
  cursor: pointer;
  white-space: nowrap;
  border-bottom: 2px solid transparent;
}
.pg-tab.active { color: var(--vp-c-text-1); border-bottom-color: var(--vp-c-brand-1); background: var(--vp-c-bg); }
.pg-close { color: var(--vp-c-text-3); font-size: 14px; line-height: 1; }
.pg-close:hover { color: var(--vp-c-danger-1); }
.pg-add { font-size: 16px; }
.pg-count {
  margin-left: 6px;
  padding: 0 6px;
  border-radius: 8px;
  background: var(--vp-c-danger-soft);
  color: var(--vp-c-danger-1);
  font-size: 12px;
}
.pg-output {
  flex: 1;
  margin: 0;
  padding: 12px;
  overflow: auto;
  font-family: var(--vp-font-family-mono);
  font-size: 13px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}
.pg-stderr, .pg-error, .pg-fail { color: var(--vp-c-danger-1); }
.pg-info { color: var(--vp-c-text-3); }
.pg-pass { color: var(--vp-c-green-1, var(--vp-c-success-1)); }
.pg-problems { flex: 1; margin: 0; padding: 0; overflow: auto; list-style: none; font-size: 13px; }
.pg-problems li { padding: 6px 12px; border-bottom: 1px solid var(--vp-c-divider); cursor: pointer; }
.pg-problems li:hover { background: var(--vp-c-default-soft); }
.pg-problems .pg-none { color: var(--vp-c-text-3); cursor: default; }
.pg-where { font-family: var(--vp-font-family-mono); color: var(--vp-c-text-2); margin-right: 6px; }
.eye-doc p { margin: 6px 0; font-size: 13px; line-height: 1.5; }
.eye-doc pre { margin: 6px 0; padding: 0; font-family: var(--vp-font-family-mono); font-size: 13px; white-space: pre-wrap; }
.eye-doc code { font-family: var(--vp-font-family-mono); font-size: 12px; background: var(--vp-c-default-soft); padding: 1px 4px; border-radius: 4px; }
</style>
