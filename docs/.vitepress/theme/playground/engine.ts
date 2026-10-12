// The page's side of the engine: a worker for the editor (checking, hover,
// completion, highlighting, formatting) and another for running programs
// and their tests, which Stop or a time limit can end and start again
// without the editor losing its answers.

export type Position = { line: number; character: number }
export type Range = { start: Position; end: Position }
export type Diagnostic = { path: string; range: Range; message: string }
export type Completion = { label: string; kind: string; detail?: string; doc?: string }
export type Location = { path: string; range: Range }
export type Span = { from: number; to: number; class: string }
export type TestResult = {
  name: string
  path: string
  line: number
  failure?: string
  skipped?: string
  log?: string
  micros: number
}
export type TestReport = {
  diagnostics: Diagnostic[]
  results: TestResult[]
  passed: number
  failed: number
  skipped: number
  output: string
}
export type RunReport = { diagnostics: Diagnostic[]; exit: number }
export type Files = Record<string, string>
export type Status = { state: 'loading' | 'ready' | 'failed'; loaded?: number; total?: number; error?: string }

type Pending = { resolve(v: any): void; reject(e: Error): void; out?: (stream: string, text: string) => void }

// One worker running eye.wasm.
class Worker_ {
  private worker: Worker
  private pending = new Map<number, Pending>()
  private next = 1
  private synced = -1 // the files' version the worker holds
  status: Status = { state: 'loading' }
  onStatus?: (s: Status) => void

  constructor(base: string) {
    this.worker = new Worker(new URL('./worker.ts', import.meta.url), { type: 'module' })
    this.worker.onmessage = (e) => this.receive(e.data)
    this.worker.onerror = (e) => this.setStatus({ state: 'failed', error: e.message || 'the engine failed to start' })
    this.worker.postMessage({ init: { base } })
  }

  private setStatus(s: Status) {
    this.status = s
    this.onStatus?.(s)
  }

  private receive(m: any) {
    if (m.progress) return this.setStatus({ state: 'loading', ...m.progress })
    if (m.ready) return this.setStatus({ state: 'ready' })
    if (m.failed) return this.setStatus({ state: 'failed', error: m.failed })
    if (m.exited) return this.setStatus({ state: 'failed', error: 'the engine stopped; reload the page' })
    const p = this.pending.get(m.id)
    if (!p) return
    if (m.out) return p.out?.(m.out.stream, m.out.text)
    this.pending.delete(m.id)
    if ('error' in m) p.reject(new Error(m.error))
    else p.resolve(m.result)
  }

  call<T>(op: string, args?: unknown, out?: Pending['out']): Promise<T> {
    const id = this.next++
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve, reject, out })
      this.worker.postMessage({ id, op, args })
    })
  }

  // sync gives the worker the files, unless it has this version already.
  sync(files: Files, version: number): Promise<unknown> | undefined {
    if (this.synced === version) return
    this.synced = version
    return this.call('setFiles', { files })
  }

  terminate(reason: string) {
    this.worker.terminate()
    for (const p of this.pending.values()) p.reject(new Error(reason))
    this.pending.clear()
  }
}

export class Engine {
  private tools: Worker_
  private runner?: Worker_
  private files: Files = {}
  private version = 0
  private base: string
  onStatus?: (s: Status) => void

  constructor(base: string) {
    this.base = base
    this.tools = new Worker_(base)
    this.tools.onStatus = (s) => this.onStatus?.(s)
  }

  get status() {
    return this.tools.status
  }

  setFiles(files: Files) {
    this.files = { ...files }
    this.version++
  }

  private async tool<T>(op: string, args?: unknown): Promise<T> {
    await this.tools.sync(this.files, this.version)
    return this.tools.call<T>(op, args)
  }

  check = () => this.tool<Diagnostic[]>('check')
  hover = (path: string, at: Position) => this.tool<string>('hover', { path, at })
  complete = (path: string, at: Position) => this.tool<Completion[]>('complete', { path, at })
  definition = (path: string, at: Position) => this.tool<Location[]>('definition', { path, at })
  format = (path: string) => this.tool<{ text: string }>('format', { path })
  highlight = (text: string) => this.tools.call<Span[]>('highlight', { text })

  private runnerWorker() {
    if (!this.runner) this.runner = new Worker_(this.base)
    return this.runner
  }

  // run runs main, then the jobs, on a clock that skips ahead. A run that
  // takes longer than limit ms, or that stop() ends, rejects.
  async run(args: string[], out: (stream: string, text: string) => void, limit = 10000): Promise<RunReport> {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone
    return this.limited(limit, (w) => w.call<RunReport>('run', { args, zone }, out))
  }

  test(filter: string, limit = 10000): Promise<TestReport> {
    return this.limited(limit, (w) => w.call<TestReport>('test', { filter }))
  }

  private async limited<T>(limit: number, f: (w: Worker_) => Promise<T>): Promise<T> {
    const w = this.runnerWorker()
    await w.sync(this.files, this.version)
    let timer: ReturnType<typeof setTimeout> | undefined
    const timeout = new Promise<never>((_, reject) => {
      timer = setTimeout(() => {
        this.stop(`stopped after ${limit / 1000} s: the program may never end`)
        reject(new Error(`stopped after ${limit / 1000} s: the program may never end`))
      }, limit)
    })
    try {
      return await Promise.race([f(w), timeout])
    } finally {
      clearTimeout(timer)
    }
  }

  // stop ends a run or test that is going; the next starts a fresh worker.
  stop(reason = 'stopped') {
    this.runner?.terminate(reason)
    this.runner = undefined
  }

  dispose() {
    this.stop()
    this.tools.terminate('closed')
  }
}
