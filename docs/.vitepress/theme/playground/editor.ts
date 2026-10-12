// The playground's editor: CodeMirror, with what the language server gives
// an editor (errors at their spans, hover, completion, go to definition,
// formatting) and highlighting from Eyelang's own parser, all answered by
// the engine in a worker.

import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  completionKeymap,
  type CompletionContext,
  type CompletionResult,
} from '@codemirror/autocomplete'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { bracketMatching, indentUnit } from '@codemirror/language'
import { linter, lintGutter, lintKeymap, type Diagnostic as LintDiagnostic } from '@codemirror/lint'
import { highlightSelectionMatches, searchKeymap } from '@codemirror/search'
import { EditorState, Facet, RangeSetBuilder, StateEffect, StateField, type Extension, type Text } from '@codemirror/state'
import {
  Decoration,
  type DecorationSet,
  drawSelection,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  hoverTooltip,
  keymap,
  lineNumbers,
  ViewPlugin,
  type ViewUpdate,
} from '@codemirror/view'
import type { Diagnostic, Engine, Location, Position } from './engine'

// The path of the file a state holds, relative to the program's root.
export const filePath = Facet.define<string, string>({ combine: (v) => v[0] ?? '' })

export function toOffset(doc: Text, p: Position): number {
  const n = Math.min(Math.max(p.line + 1, 1), doc.lines)
  const line = doc.line(n)
  return Math.min(line.from + Math.max(p.character, 0), line.to)
}

export function toPosition(doc: Text, offset: number): Position {
  const line = doc.lineAt(offset)
  return { line: line.number - 1, character: offset - line.from }
}

// ---------------------------------------------------------------- highlighting

const setSpans = StateEffect.define<DecorationSet>()

const spansField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    for (const e of tr.effects) if (e.is(setSpans)) return e.value
    return deco.map(tr.changes)
  },
  provide: (f) => EditorView.decorations.from(f),
})

const marks: Record<string, Decoration> = {}
const mark = (cls: string) => (marks[cls] ??= Decoration.mark({ class: `eye-${cls}` }))

function highlighter(engine: Engine): Extension {
  const plugin = ViewPlugin.fromClass(
    class {
      timer: ReturnType<typeof setTimeout> | undefined
      constructor(readonly view: EditorView) {
        this.schedule(0)
      }
      update(u: ViewUpdate) {
        if (u.docChanged) this.schedule(40)
      }
      schedule(delay: number) {
        clearTimeout(this.timer)
        this.timer = setTimeout(() => this.run(), delay)
      }
      async run() {
        const doc = this.view.state.doc
        let spans
        try {
          spans = await engine.highlight(doc.toString())
        } catch {
          return
        }
        if (this.view.state.doc !== doc) return // a newer request is coming
        const b = new RangeSetBuilder<Decoration>()
        for (const s of spans) if (s.to > s.from && s.to <= doc.length) b.add(s.from, s.to, mark(s.class))
        this.view.dispatch({ effects: setSpans.of(b.finish()) })
      }
      destroy() {
        clearTimeout(this.timer)
      }
    },
  )
  return [spansField, plugin]
}

// ---------------------------------------------------------------- hover

const escape = (s: string) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!)

// inline renders `code`, **bold** and [text](link) in escaped text; links to
// lines of eye's own source go nowhere in a browser, so only their text stays.
function inline(s: string): string {
  return escape(s)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
}

// markdown renders the Markdown hover and completion give: fenced code
// blocks, highlighted by the engine, and paragraphs.
export async function markdown(engine: Engine, md: string): Promise<HTMLElement> {
  const root = document.createElement('div')
  root.className = 'eye-doc'
  const parts = md.split(/```(\w*)\n([\s\S]*?)```/g)
  for (let i = 0; i < parts.length; i += 3) {
    for (const para of parts[i].split(/\n\s*\n/)) {
      if (!para.trim()) continue
      const p = document.createElement('p')
      p.innerHTML = inline(para.trim())
      root.append(p)
    }
    if (i + 2 < parts.length) {
      const code = parts[i + 2].replace(/\n$/, '')
      const pre = document.createElement('pre')
      pre.innerHTML = parts[i + 1] === 'eye' ? await highlighted(engine, code) : escape(code)
      root.append(pre)
    }
  }
  return root
}

async function highlighted(engine: Engine, code: string): Promise<string> {
  let spans: { from: number; to: number; class: string }[] = []
  try {
    spans = await engine.highlight(code)
  } catch {}
  let out = ''
  let at = 0
  for (const s of spans) {
    out += escape(code.slice(at, s.from)) + `<span class="eye-${s.class}">${escape(code.slice(s.from, s.to))}</span>`
    at = s.to
  }
  return out + escape(code.slice(at))
}

function hover(engine: Engine): Extension {
  return hoverTooltip(async (view, pos) => {
    const path = view.state.facet(filePath)
    if (!path.endsWith('.eye')) return null
    const word = view.state.wordAt(pos)
    let md: string
    try {
      md = await engine.hover(path, toPosition(view.state.doc, pos))
    } catch {
      return null
    }
    if (!md) return null
    const dom = await markdown(engine, md)
    return { pos: word?.from ?? pos, end: word?.to ?? pos, above: true, create: () => ({ dom }) }
  })
}

// ---------------------------------------------------------------- completion

const completionTypes: Record<string, string> = {
  keyword: 'keyword', function: 'function', method: 'method', field: 'property', variable: 'variable',
  type: 'type', trait: 'interface', module: 'namespace', enum: 'enum', variant: 'enum', event: 'constant',
  text: 'text',
}

function completion(engine: Engine): Extension {
  const source = async (ctx: CompletionContext): Promise<CompletionResult | null> => {
    const path = ctx.state.facet(filePath)
    if (!path.endsWith('.eye')) return null
    const word = ctx.matchBefore(/[A-Za-z_][A-Za-z0-9_]*/)
    const afterDot = ctx.matchBefore(/\.\s*$/) !== null || (word !== null && ctx.state.sliceDoc(word.from - 1, word.from) === '.')
    if (!word && !afterDot && !ctx.explicit) return null
    let items
    try {
      items = await engine.complete(path, toPosition(ctx.state.doc, ctx.pos))
    } catch {
      return null
    }
    if (ctx.aborted || !items.length) return null
    return {
      from: word?.from ?? ctx.pos,
      validFor: /^[A-Za-z0-9_]*$/,
      options: items.map((it) => ({
        label: it.label,
        type: completionTypes[it.kind] ?? 'text',
        detail: it.detail,
        info: it.doc ? () => markdown(engine, it.doc!) : undefined,
      })),
    }
  }
  return autocompletion({ override: [source], activateOnTyping: true })
}

// ---------------------------------------------------------------- diagnostics

function diagnostics(engine: Engine, onProblems: (all: Diagnostic[]) => void): Extension {
  return linter(
    async (view) => {
      const path = view.state.facet(filePath)
      let all
      try {
        all = await engine.check()
      } catch {
        return []
      }
      onProblems(all)
      const doc = view.state.doc
      return all
        .filter((d) => d.path === path)
        .map((d): LintDiagnostic => {
          const from = toOffset(doc, d.range.start)
          const to = Math.max(from, toOffset(doc, d.range.end))
          return { from, to, severity: 'error', message: d.message, source: 'eye' }
        })
    },
    { delay: 350 },
  )
}

// ---------------------------------------------------------------- the editor

export type Hooks = {
  onChange(path: string, text: string): void
  onRun(): void
  onTest(): void
  onFormat(): void
  onJump(loc: Location): void
  onProblems(all: Diagnostic[]): void
}

const theme = EditorView.theme({
  '&': { height: '100%', fontSize: '14px', backgroundColor: 'var(--vp-c-bg)', color: 'var(--vp-c-text-1)' },
  '.cm-scroller': { fontFamily: 'var(--vp-font-family-mono)', lineHeight: '1.6' },
  '.cm-gutters': { backgroundColor: 'var(--vp-c-bg-soft)', color: 'var(--vp-c-text-3)', border: 'none' },
  '.cm-activeLine': { backgroundColor: 'var(--vp-c-default-soft)' },
  '.cm-activeLineGutter': { backgroundColor: 'var(--vp-c-default-soft)' },
  '&.cm-focused': { outline: 'none' },
  '.cm-content': { caretColor: 'var(--vp-c-brand-1)' },
  '.cm-cursor': { borderLeftColor: 'var(--vp-c-brand-1)' },
  '.cm-tooltip': {
    backgroundColor: 'var(--vp-c-bg-elv)',
    border: '1px solid var(--vp-c-divider)',
    borderRadius: '6px',
    boxShadow: 'var(--vp-shadow-3)',
  },
  '.cm-tooltip-hover': { maxWidth: '560px', maxHeight: '320px', overflow: 'auto', padding: '4px 10px' },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection': {
    backgroundColor: 'var(--vp-c-brand-soft) !important',
  },
})

export function createState(engine: Engine, path: string, text: string, hooks: Hooks): EditorState {
  const eye = path.endsWith('.eye')
  const definitionAt = async (view: EditorView, pos: number) => {
    if (!eye) return false
    const locs = await engine.definition(path, toPosition(view.state.doc, pos)).catch(() => [])
    if (locs.length) hooks.onJump(locs[0])
    return locs.length > 0
  }
  return EditorState.create({
    doc: text,
    extensions: [
      filePath.of(path),
      lineNumbers(),
      highlightActiveLineGutter(),
      history(),
      drawSelection(),
      EditorState.allowMultipleSelections.of(true),
      EditorState.tabSize.of(2),
      indentUnit.of('  '),
      bracketMatching(),
      closeBrackets(),
      highlightActiveLine(),
      highlightSelectionMatches(),
      theme,
      keymap.of([
        { key: 'Mod-Enter', run: () => (hooks.onRun(), true) },
        { key: 'Mod-Shift-Enter', run: () => (hooks.onTest(), true) },
        { key: 'Shift-Alt-f', run: () => (hooks.onFormat(), true) },
        { key: 'Mod-s', run: () => (hooks.onFormat(), true), preventDefault: true },
        { key: 'F12', run: (view) => (definitionAt(view, view.state.selection.main.head), true) },
        ...closeBracketsKeymap,
        ...defaultKeymap,
        ...searchKeymap,
        ...historyKeymap,
        ...completionKeymap,
        ...lintKeymap,
        indentWithTab,
      ]),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) hooks.onChange(path, u.state.doc.toString())
      }),
      EditorView.domEventHandlers({
        mousedown(e, view) {
          if (!(e.metaKey || e.ctrlKey) || e.button !== 0) return false
          const pos = view.posAtCoords({ x: e.clientX, y: e.clientY })
          if (pos == null) return false
          e.preventDefault()
          definitionAt(view, pos)
          return true
        },
      }),
      ...(eye ? [highlighter(engine), hover(engine), completion(engine), diagnostics(engine, hooks.onProblems), lintGutter()] : []),
    ],
  })
}
