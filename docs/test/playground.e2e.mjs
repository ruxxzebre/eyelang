// The playground, end to end: serve the built site (task docs), open
// /playground in Chromium, and check that the engine loads and that a
// program checks, runs, tests and formats there, with hover and completion.
//
// The browser is $CHROMIUM if set, then Playwright's (PLAYWRIGHT_BROWSERS_PATH
// or /opt/pw-browsers), then Chrome or Chromium on PATH, then Playwright's "chrome" channel.

import { createServer } from 'node:http'
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { extname, join, normalize } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'

const dist = fileURLToPath(new URL('../.vitepress/dist', import.meta.url))
if (!existsSync(join(dist, 'wasm', 'eye.wasm'))) {
  console.error('playground.e2e: no built site with the engine in docs/.vitepress/dist: run task docs first')
  process.exit(1)
}

const types = {
  '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.wasm': 'application/wasm',
  '.json': 'application/json', '.svg': 'image/svg+xml', '.woff2': 'font/woff2',
}
const server = createServer((req, res) => {
  let path = normalize(decodeURIComponent(new URL(req.url, 'http://x').pathname)).replace(/^(\.\.[/\\])+/, '')
  let file = join(dist, path)
  if (existsSync(file) && statSync(file).isDirectory()) file = join(file, 'index.html')
  if (!existsSync(file) && existsSync(file + '.html')) file = file + '.html'
  if (!existsSync(file)) {
    res.writeHead(404).end('not found')
    return
  }
  res.writeHead(200, { 'content-type': types[extname(file)] ?? 'application/octet-stream' })
  res.end(readFileSync(file))
})
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const base = `http://127.0.0.1:${server.address().port}`

function browserPath() {
  if (process.env.CHROMIUM) return process.env.CHROMIUM
  for (const dir of [process.env.PLAYWRIGHT_BROWSERS_PATH, '/opt/pw-browsers'].filter(Boolean)) {
    if (!existsSync(dir)) continue
    for (const d of readdirSync(dir).filter((d) => /^chromium-\d+$/.test(d)).sort().reverse()) {
      for (const exe of ['chrome-linux/chrome', 'chrome-linux64/chrome', 'chrome-mac/Chromium.app/Contents/MacOS/Chromium']) {
        if (existsSync(join(dir, d, exe))) return join(dir, d, exe)
      }
    }
  }
  for (const dir of (process.env.PATH ?? '').split(':')) {
    for (const name of ['google-chrome', 'chromium', 'chromium-browser']) {
      if (dir && existsSync(join(dir, name))) return join(dir, name)
    }
  }
  return undefined
}

const exe = browserPath()
const browser = await chromium.launch(exe ? { executablePath: exe } : { channel: 'chrome' })
const page = await browser.newPage()
const errors = []
page.on('pageerror', (e) => errors.push(e.message))
page.on('console', (m) => m.type() === 'error' && errors.push(m.text()))

let failed = 0
async function step(name, f) {
  const started = Date.now()
  try {
    await f()
    console.log(`ok    ${name} (${Date.now() - started} ms)`)
  } catch (e) {
    failed++
    console.log(`FAIL  ${name}\n      ${String(e.message).split('\n').join('\n      ')}`)
  }
}

const output = () => page.locator('.pg-output').innerText()
const editor = page.locator('.pg-code .cm-content')
// setCode replaces the open file's text as typing would.
async function setCode(text) {
  await editor.click()
  await page.keyboard.press('ControlOrMeta+a')
  await page.keyboard.press('Delete')
  await page.keyboard.insertText(text)
}
const code = () => editor.innerText()

try {
  await step('the engine loads', async () => {
    await page.goto(`${base}/playground#example=tutorial/hello`)
    await page.locator('.pg-primary:not([disabled])').waitFor({ timeout: 60_000 })
  })

  await step('Run prints what main prints, with arguments', async () => {
    await page.locator('.pg-args input').fill('Ann Bo')
    await page.locator('.pg-primary').click()
    await page.waitForFunction(() => /exit \d/.test(document.querySelector('.pg-output')?.textContent ?? ''), null, { timeout: 30_000 })
    const out = await output()
    if (!out.includes('Hello, Ann!\nHello, Bo!') || !out.includes('exit 0')) throw new Error(`output:\n${out}`)
  })

  await step('Test runs the tests', async () => {
    await page.getByRole('button', { name: 'Test', exact: true }).click()
    await page.waitForFunction(() => /tests? (passed|failed)/.test(document.querySelector('.pg-output')?.textContent ?? ''), null, { timeout: 30_000 })
    const out = await output()
    if (!out.includes('ok    greeting names who it greets') || !out.includes('1 tests passed')) throw new Error(`output:\n${out}`)
  })

  await step('the parser highlights the code', async () => {
    await page.locator('.pg-code .eye-k').first().waitFor({ timeout: 10_000 })
    const kw = await page.locator('.pg-code .eye-k').first().innerText()
    if (!['fn', 'pub', 'let', 'if', 'for', 'in'].includes(kw)) throw new Error(`first keyword: ${kw}`)
  })

  await step('a type error is underlined and listed', async () => {
    await setCode('fn greeting(name: String) -> String = "Hello, {name}!"\n\npub fn main() uses {stdio} {\n  print(greeting(42))\n}\n')
    await page.locator('.pg-code .cm-lintRange-error').first().waitFor({ timeout: 10_000 })
    await page.getByRole('tab', { name: /Problems/ }).click()
    const text = await page.locator('.pg-problems').innerText()
    if (!text.includes('main.eye:4:18') || !text.includes('expected String, found Int')) throw new Error(`problems:\n${text}`)
  })

  await step('hover shows a signature', async () => {
    // The middle of the word, found in the line's text.
    const at = await page.evaluate(() => {
      const line = document.querySelectorAll('.pg-code .cm-line')[3]
      const walk = document.createTreeWalker(line, NodeFilter.SHOW_TEXT)
      for (let n = walk.nextNode(); n; n = walk.nextNode()) {
        const i = n.textContent.indexOf('greeting')
        if (i < 0) continue
        const r = document.createRange()
        r.setStart(n, i + 2)
        r.setEnd(n, i + 3)
        const box = r.getBoundingClientRect()
        return { x: box.x + box.width / 2, y: box.y + box.height / 2 }
      }
    })
    await page.mouse.move(at.x, at.y)
    const tip = page.locator('.cm-tooltip-hover')
    await tip.waitFor({ timeout: 10_000 })
    const text = await tip.innerText()
    if (!text.includes('fn greeting(name: String) -> String')) throw new Error(`hover:\n${text}`)
  })

  await step('completion offers the names in scope', async () => {
    await setCode('fn greeting(name: String) -> String = "Hello, {name}!"\n\npub fn main() uses {stdio} {\n  gree\n}\n')
    await page.locator('.pg-code .cm-line').nth(3).click()
    await page.keyboard.press('End')
    await page.keyboard.press('Control+Space')
    const option = page.locator('.cm-tooltip-autocomplete li', { hasText: 'greeting' })
    await option.first().waitFor({ timeout: 10_000 })
  })

  await step('Format rewrites the file in the canonical style', async () => {
    await page.keyboard.press('Escape')
    await setCode('fn  shout( s:String )->String=s+"!"\n')
    await page.getByRole('button', { name: 'Format', exact: true }).click()
    await page.locator('.pg-code .cm-line', { hasText: 'fn shout(s: String) -> String = s + "!"' }).waitFor({ timeout: 10_000 })
  })

  await step('a job that sleeps 15 minutes runs on the virtual clock', async () => {
    await page.locator('.pg-select').selectOption('tutorial/a-job-that-waits')
    await page.locator('.pg-primary').click()
    await page.waitForFunction(() => /exit \d/.test(document.querySelector('.pg-output')?.textContent ?? ''), null, { timeout: 30_000 })
    const out = await output()
    if (!/released order-1's hold on apples/.test(out) || !out.includes('exit 0')) throw new Error(`output:\n${out}`)
  })

  await step('a program with modules and recordings passes its tests', async () => {
    await page.locator('.pg-select').selectOption('tutorial/approvals')
    if ((await page.locator('.pg-tab', { hasText: 'notify/notify.eye' }).count()) !== 1) throw new Error('no tab for notify/notify.eye')
    await page.getByRole('button', { name: 'Test', exact: true }).click()
    await page.waitForFunction(() => /tests? (passed|failed)/.test(document.querySelector('.pg-output')?.textContent ?? ''), null, { timeout: 30_000 })
    const out = await output()
    if (!/\d+ tests passed/.test(out)) throw new Error(`output:\n${out}`)
  })

  await step('a program calls a contract another program serves, in the page', async () => {
    await page.locator('.pg-select').selectOption('examples/shop-and-bank')
    if ((await page.locator('.pg-tab', { hasText: 'bank/bank.eye' }).count()) !== 1) throw new Error('no tab for bank/bank.eye')
    await page.locator('.pg-primary').click()
    await page.waitForFunction(() => /exit \d/.test(document.querySelector('.pg-output')?.textContent ?? ''), null, { timeout: 30_000 })
    const out = await output()
    for (const want of [
      'order 1: paid 2500 cents, 7500 left',
      'order 2: declined: only 7500 cents left',
      'bank │ payments.Payments POST /v1/charges → 201 Created',
      'bank │ payments.Payments POST /v1/charges → 402 Payment Required',
      'bank │ payments.charged POST /events/payments.charged → 202 Accepted',
      'receipt for order-1: 2500 cents',
      'ann has 7500 cents',
      'exit 0',
    ]) {
      if (!out.includes(want)) throw new Error(`no "${want}" in the output:\n${out}`)
    }
    if ((await page.locator('.pg-output .pg-service').count()) === 0) throw new Error('the bank\'s lines aren\'t marked as a service\'s')
    // Hover in the bank's program knows the contract the other directory declares.
    await page.locator('.pg-tab', { hasText: 'bank/bank.eye' }).click()
    await page.getByRole('tab', { name: /Problems/ }).click()
    const problems = await page.locator('.pg-problems').innerText()
    if (!problems.includes('No problems.')) throw new Error(`problems:\n${problems}`)
  })

  await step('Stop ends a program that never ends', async () => {
    await page.locator('.pg-select').selectOption('blank')
    await setCode('pub fn main() uses {stdio} {\n  var n = 0\n  while true { n += 1 }\n}\n')
    await page.locator('.pg-primary').click()
    await page.getByRole('button', { name: 'Stop' }).click({ timeout: 5_000 })
    await page.waitForFunction(() => /stopped/.test(document.querySelector('.pg-output')?.textContent ?? ''), null, { timeout: 10_000 })
    // The next run starts a fresh engine.
    await setCode('pub fn main() uses {stdio} { print("again") }\n')
    await page.locator('.pg-primary:not([disabled])').click()
    await page.waitForFunction(() => /exit \d/.test(document.querySelector('.pg-output')?.textContent ?? ''), null, { timeout: 60_000 })
    if (!(await output()).includes('again')) throw new Error(`output:\n${await output()}`)
  })

  await step('a shared link holds the program', async () => {
    await page.context().grantPermissions(['clipboard-read', 'clipboard-write'])
    await page.getByRole('button', { name: 'Share' }).click()
    await page.waitForFunction(() => location.hash.startsWith('#code='), null, { timeout: 5_000 })
    const url = page.url()
    const other = await browser.newPage()
    await other.goto(url)
    await other.locator('.pg-code .cm-content').waitFor()
    const text = await other.locator('.pg-code .cm-content').innerText()
    await other.close()
    if (!text.includes('print("again")')) throw new Error(`opened:\n${text}`)
  })

  await step('no errors in the console', async () => {
    const real = errors.filter((e) => !/Failed to load resource/.test(e))
    if (real.length) throw new Error(real.join('\n'))
  })
} finally {
  await browser.close()
  server.close()
}

console.log(failed ? `${failed} playground checks failed` : 'the playground works')
process.exit(failed ? 1 : 0)
