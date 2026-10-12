// The playground's engine runs here, off the page's thread: the checker,
// interpreter, formatter and language server, compiled to WebAssembly from
// cmd/eye-wasm. The page sends {id, op, args} and gets {id, result} or
// {id, error} back; a run also sends {id, out: {stream, text}} as the
// program prints. A program that never stops is stopped by ending the
// worker (engine.ts).

declare const self: DedicatedWorkerGlobalScope

type Init = { init: { base: string } }
type Request = { id: number; op: string; args?: unknown }

declare global {
  // Set by wasm_exec.js and by eye-wasm.
  var Go: new () => { importObject: WebAssembly.Imports; run(i: WebAssembly.Instance): Promise<void> }
  var eye: { call(op: string, args: string, out?: (stream: string, text: string) => void): string }
  var eyeReady: (() => void) | undefined
}

async function load(base: string) {
  // wasm_exec.js is a plain script that sets globalThis.Go.
  await import(/* @vite-ignore */ `${base}wasm/wasm_exec.js`)
  const response = await fetch(`${base}wasm/eye.wasm`)
  if (!response.ok || !response.body) throw new Error(`eye.wasm: ${response.status} ${response.statusText}`)
  // Read it in pieces, so the page can say how much has come.
  const total = Number(response.headers.get('content-length')) || 0
  const reader = response.body.getReader()
  const chunks: Uint8Array[] = []
  let loaded = 0
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    chunks.push(value)
    loaded += value.length
    self.postMessage({ progress: { loaded, total } })
  }
  const bytes = new Uint8Array(loaded)
  let at = 0
  for (const c of chunks) {
    bytes.set(c, at)
    at += c.length
  }
  const go = new globalThis.Go()
  const ready = new Promise<void>((resolve) => (globalThis.eyeReady = resolve))
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject)
  go.run(instance).then(() => self.postMessage({ exited: true }))
  await ready
}

let loaded: Promise<void> | undefined

self.onmessage = async (e: MessageEvent<Init | Request>) => {
  if ('init' in e.data) {
    loaded = load(e.data.init.base)
    loaded.then(
      () => self.postMessage({ ready: true }),
      (err) => self.postMessage({ failed: String(err?.message ?? err) }),
    )
    return
  }
  const { id, op, args } = e.data
  try {
    await loaded
    const out = (stream: string, text: string) => self.postMessage({ id, out: { stream, text } })
    const reply = JSON.parse(globalThis.eye.call(op, JSON.stringify(args ?? {}), out))
    self.postMessage({ id, ...reply })
  } catch (err: any) {
    self.postMessage({ id, error: String(err?.message ?? err) })
  }
}
