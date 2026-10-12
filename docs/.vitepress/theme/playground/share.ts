// Where the playground's program comes from when the page opens, and where
// it goes: a link's #code= (the files, compressed), a link's #example=, or
// what the visitor last had, kept in this browser.

import type { Files } from './engine'

const key = 'eyelang-playground'

export type Saved = { files: Files; active: string; args: string; example: string }

export function load(): Partial<Saved> | undefined {
  try {
    const raw = localStorage.getItem(key)
    return raw ? JSON.parse(raw) : undefined
  } catch {
    return undefined
  }
}

export function save(s: Saved) {
  try {
    localStorage.setItem(key, JSON.stringify(s))
  } catch {}
}

const toBase64url = (bytes: Uint8Array) => {
  let s = ''
  for (const b of bytes) s += String.fromCharCode(b)
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

const fromBase64url = (s: string) => {
  const bin = atob(s.replace(/-/g, '+').replace(/_/g, '/'))
  return Uint8Array.from(bin, (c) => c.charCodeAt(0))
}

async function pipe(bytes: Uint8Array, stream: CompressionStream | DecompressionStream): Promise<Uint8Array> {
  const out = new Response(new Blob([bytes]).stream().pipeThrough(stream))
  return new Uint8Array(await out.arrayBuffer())
}

// encode makes the #code= of a link to these files.
export async function encode(files: Files): Promise<string> {
  const json = new TextEncoder().encode(JSON.stringify(files))
  return toBase64url(await pipe(json, new CompressionStream('deflate-raw')))
}

export async function decode(code: string): Promise<Files> {
  const json = await pipe(fromBase64url(code), new DecompressionStream('deflate-raw'))
  const files = JSON.parse(new TextDecoder().decode(json))
  if (typeof files !== 'object' || files === null) throw new Error('not a program')
  for (const [k, v] of Object.entries(files)) if (typeof v !== 'string') throw new Error(`${k} isn't text`)
  return files as Files
}

// fromHash reads the page's #example= or #code=.
export async function fromHash(hash: string): Promise<{ example?: string; files?: Files } | undefined> {
  const params = new URLSearchParams(hash.replace(/^#/, ''))
  if (params.get('code')) {
    try {
      return { files: await decode(params.get('code')!) }
    } catch {
      return undefined
    }
  }
  if (params.get('example')) return { example: params.get('example')! }
  return undefined
}
