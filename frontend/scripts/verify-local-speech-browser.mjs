// Read-aloud's voice model on this device (#204), in a browser without a GPU: a stub engine
// worker speaks the real worker protocol and answers each line with a sine tone. Uses its own
// production demo and disposable account; never targets the live backend.
//
// Run after `bun run build`, from frontend/: bun scripts/verify-local-speech-browser.mjs
// The real model needs a GPU; docs/frontend.md describes that check.
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { createServer } from 'node:net'
import { chromium } from 'playwright'

const stubWorker = (mode) => `
const bc = new BroadcastChannel('ircord-voice-stub')
const aborted = new Set()
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
self.onmessage = async (event) => {
  const m = event.data
  if (m.type === 'init') {
    bc.postMessage({ type: 'init' })
    if (${JSON.stringify(mode)} === 'fail') return postMessage({ type: 'fatal', message: 'stub adapter lost' })
    // "Download": the shards the shipped manifests name go into the engine's cache.
    const cache = await caches.open('zonos3-web-models-v6')
    const shards = []
    for (const model of ['backbone', 'zac']) {
      const manifest = await (await fetch(new URL(model + '/manifest.json', m.base))).json()
      for (const name of manifest.shards) shards.push(new URL(model + '/' + name, m.base).href)
    }
    let loaded = 0
    let fetched = 0
    for (const url of shards) {
      if (!(await cache.match(url))) {
        await cache.put(url, new Response('x'))
        fetched += 1000
      }
      loaded += 1000
      postMessage({ type: 'progress', models: ['backbone', 'zac'], loaded, total: shards.length * 1000, fetched })
      await sleep(30)
    }
    postMessage({ type: 'ready' })
  } else if (m.type === 'req') {
    const body = JSON.parse(new TextDecoder().decode(m.body))
    bc.postMessage({ type: 'req', body })
    postMessage({ type: 'res', id: m.id, status: 200, headers: { 'content-type': 'audio/pcm' } })
    let n = 0
    for (let c = 0; c < 6 && !aborted.has(m.id); c++) {
      const pcm = new Float32Array(8820)
      for (let i = 0; i < pcm.length; i++, n++) pcm[i] = 0.05 * Math.sin((2 * Math.PI * 440 * n) / 44100)
      postMessage({ type: 'chunk', id: m.id, data: pcm.buffer }, [pcm.buffer])
      await sleep(15)
    }
    postMessage({ type: 'end', id: m.id })
  } else if (m.type === 'abort') aborted.add(m.id)
}`
const stubCodec = `self.onmessage = () => {}`

// A browser with WebGPU on a real adapter, as far as the page can tell; and the stub's reports.
const fakeGpu = () => {
  Object.defineProperty(navigator, 'gpu', {
    value: { requestAdapter: async () => ({ info: { isFallbackAdapter: false } }) },
  })
}
const softwareGpu = () => {
  Object.defineProperty(navigator, 'gpu', {
    value: { requestAdapter: async () => ({ info: { isFallbackAdapter: true } }) },
  })
}
const watchStub = () => {
  window.__stub = { inits: 0, requests: [] }
  new BroadcastChannel('ircord-voice-stub').onmessage = (event) => {
    if (event.data.type === 'init') window.__stub.inits++
    else window.__stub.requests.push(event.data.body)
  }
  window.__workers = { created: 0, terminated: 0 }
  const terminate = Worker.prototype.terminate
  const Original = Worker
  // Only the voice model's two workers; the app has others (offline search).
  const voice = new WeakSet()
  window.Worker = class extends Original {
    constructor(url, options) {
      super(url, options)
      if (String(url).includes('/speech/')) {
        voice.add(this)
        window.__workers.created++
      }
    }
  }
  Worker.prototype.terminate = function () {
    if (voice.has(this)) window.__workers.terminated++
    return terminate.call(this)
  }
}

const portProbe = createServer()
await new Promise((resolve) => portProbe.listen(0, '127.0.0.1', resolve))
const port = portProbe.address().port
await new Promise((resolve) => portProbe.close(resolve))
const base = `http://127.0.0.1:${port}`
const frontend = spawn(process.execPath, ['.output/server/index.mjs'], {
  env: {
    ...process.env,
    HOST: '127.0.0.1',
    PORT: String(port),
    NUXT_DEMO: 'true',
    NUXT_PUBLIC_API_BASE: '/api/v1',
  },
  stdio: ['ignore', 'pipe', 'pipe'],
})
let serverLog = ''
frontend.stdout.on('data', (chunk) => (serverLog = (serverLog + chunk).slice(-4000)))
frontend.stderr.on('data', (chunk) => (serverLog = (serverLog + chunk).slice(-4000)))
let browser
let context
let created = false
const errors = []
try {
  for (let attempt = 0; ; attempt++) {
    try {
      if ((await fetch(base)).ok) break
    } catch {
      // Not accepting connections yet.
    }
    assert(attempt < 100 && frontend.exitCode === null, 'Isolated frontend did not start')
    await new Promise((resolve) => setTimeout(resolve, 100))
  }

  // ---- Server: policy, worker policy, and the shard redirect ---------------------------------
  const policy = (await fetch(base)).headers.get('content-security-policy') ?? ''
  assert.match(policy, /connect-src [^;]*https:\/\/huggingface\.co https:\/\/\*\.hf\.co/)
  const workerFile = await fetch(`${base}/speech/zonos3/v1/worker.js`)
  assert.equal(workerFile.status, 200)
  assert.match(
    workerFile.headers.get('content-security-policy') ?? '',
    /connect-src 'self' https:\/\/huggingface\.co https:\/\/\*\.hf\.co/,
  )
  assert.match(workerFile.headers.get('cache-control') ?? '', /immutable/)
  assert.equal((await fetch(`${base}/speech/zonos3/v1/models/voices/voices.json`)).status, 200)
  const shard = await fetch(`${base}/speech/zonos3/v1/models/backbone/shard-000.bin`, {
    redirect: 'manual',
  })
  assert.equal(shard.status, 302)
  assert.match(
    shard.headers.get('location') ?? '',
    /^https:\/\/huggingface\.co\/Zyphra\/ZONOS3-Mini\/resolve\/[0-9a-f]{40}\/web\/backbone\/shard-000\.bin$/,
  )
  assert.equal(shard.headers.get('referrer-policy'), 'no-referrer')
  assert.equal(shard.headers.get('cache-control'), 'no-store')
  for (const path of [
    'models/edit/manifest.json',
    'models/encoder/shard-000.bin',
    'models/zac/shard-099.bin',
  ]) {
    const missing = await fetch(`${base}/speech/zonos3/v1/${path}`, { redirect: 'manual' })
    assert.equal(missing.status, 404, path)
  }

  browser = await chromium.launch({
    headless: true,
    args: ['--autoplay-policy=no-user-gesture-required'],
  })
  context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  let stubMode = 'ok'
  const speechRequests = []
  await context.route('**/speech/zonos3/v1/worker.js', (route) =>
    route.fulfill({
      contentType: 'text/javascript',
      headers: { 'cache-control': 'no-store' },
      body: stubWorker(stubMode),
    }),
  )
  await context.route('**/speech/zonos3/v1/codec-worker.js', (route) =>
    route.fulfill({ contentType: 'text/javascript', body: stubCodec }),
  )
  await context.route('https://speech.invalid/**', (route) => {
    speechRequests.push(route.request().url())
    return route.abort()
  })
  await context.addInitScript(watchStub)
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `voice${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  created = true

  // ---- Software-only WebGPU: the switch stays off, the downsides still open ------------------
  const soft = await context.newPage()
  soft.setDefaultTimeout(8000)
  await soft.addInitScript(softwareGpu)
  soft.on('pageerror', (error) => errors.push(error.message))
  await soft.goto(base)
  await soft.waitForURL(`${base}/channel/general`)
  const openVoiceSettings = async (page) => {
    await page.getByRole('button', { name: 'User settings' }).click()
    const settings = page.getByRole('dialog', { name: 'Settings' })
    await settings.getByRole('combobox', { name: 'Search settings' }).fill('webgpu')
    await settings.getByRole('option', { name: /Voice model on this device/ }).click()
    await settings.locator('[data-setting="local-speech"]').waitFor()
    return settings
  }
  const closeSettings = async (page) => {
    await page.keyboard.press('Escape')
    await page.getByRole('dialog', { name: 'Settings' }).waitFor({ state: 'hidden' })
  }
  let settings = await openVoiceSettings(soft)
  const softRow = settings.locator('[data-setting="local-speech"]')
  await softRow
    .getByText('Not available: this browser or graphics card has no WebGPU support.')
    .waitFor()
  assert.equal(await softRow.getByRole('switch').isDisabled(), true)
  await softRow.getByRole('button', { name: 'Before you switch this on' }).click()
  const softInfo = soft.getByRole('dialog', { name: 'Before you switch this on' })
  await softInfo.getByText(/downloads a 921 MB voice model from huggingface\.co/).waitFor()
  await softInfo
    .getByText(/IRCord does not send message text to a speech service on its own/)
    .waitFor()
  await soft.keyboard.press('Escape')
  await soft.close()

  // ---- Enabling shows the downsides first and downloads nothing ------------------------------
  const page = await context.newPage()
  page.setDefaultTimeout(8000)
  // Live events reach this tab late while `delayEvents` is set, so another tab claims a
  // highlight first unless the model lock stops it.
  let delayEvents = 0
  await page.routeWebSocket(/\/api\/v1\/events/, (ws) => {
    const server = ws.connectToServer()
    server.onMessage((message) => {
      if (delayEvents) setTimeout(() => ws.send(message), delayEvents)
      else ws.send(message)
    })
    ws.onMessage((message) => server.send(message))
  })
  await page.addInitScript(fakeGpu)
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto(base)
  await page.waitForURL(`${base}/channel/general`)
  const topBar = page.getByRole('button', { name: 'Read highlights aloud', exact: true })
  assert.equal(await topBar.count(), 0, 'no read-aloud button before anything is set up')
  settings = await openVoiceSettings(page)
  const localRow = settings.locator('[data-setting="local-speech"]')
  await localRow.getByText(/It needs no speech service and no API key/).waitFor()
  await localRow.getByRole('switch').click()
  const info = page.getByRole('dialog', { name: 'Before you switch this on' })
  await info.waitFor()
  for (const topic of [
    'Download.',
    'Memory.',
    'Battery and heat.',
    'Slower devices.',
    'No WebGPU.',
    'If the model cannot run.',
  ])
    await info.getByText(topic, { exact: true }).waitFor()
  await page.keyboard.press('Escape')
  await info.waitFor({ state: 'hidden' })
  assert.deepEqual(await page.evaluate(() => window.__workers), { created: 0, terminated: 0 })
  // Only the first time: off and on again opens nothing.
  await localRow.getByRole('switch').click()
  await localRow.getByRole('switch').click()
  await page.waitForTimeout(300)
  assert.equal(await info.count(), 0)

  const model = settings.locator('[data-setting="local-model"]')
  await model.getByText('Not downloaded. 921 MB from huggingface.co.').waitFor()
  const highlights = settings.locator('[data-setting="read-aloud-device"]')
  await highlights.getByText('Download the voice model below first.').waitFor()
  assert.equal(await highlights.getByRole('switch').isDisabled(), true)
  const voice = settings.locator('[data-setting="local-voice"]').getByRole('combobox')
  assert.equal((await voice.textContent()).trim(), 'Rupert, English (British), male')

  // ---- Download, then Test with the default voice and another ---------------------------------
  await model.getByRole('button', { name: 'Download', exact: true }).click()
  await model.getByText('Downloaded. 921 MB stored in this browser.').waitFor()
  assert.equal(await page.evaluate(() => window.__stub.inits), 1)
  const test = settings.locator('[data-setting="local-test"]')
  await test.getByRole('button', { name: 'Test', exact: true }).click()
  await test.getByText('Played a sample.').waitFor()
  let requests = await page.evaluate(() => window.__stub.requests)
  assert.equal(requests.length, 1)
  assert.equal(requests[0].text, 'This is how IRCord reads your highlights aloud.')
  assert.equal(requests[0].speaker_embedding_id, 'default_3765f0d1140c05c0')
  assert.equal(requests[0].language, 'en_gb')
  await voice.click()
  await page.getByRole('option', { name: 'Liesel, German, female' }).click()
  assert.equal((await voice.textContent()).trim(), 'Liesel, German, female')
  await test.getByRole('button', { name: 'Test', exact: true }).click()
  await test.getByText('Played a sample.').waitFor()
  requests = await page.evaluate(() => window.__stub.requests)
  assert.equal(requests.at(-1).speaker_embedding_id, 'default_09d206a5b067ae71')
  assert.equal(requests.at(-1).language, 'de')
  assert.equal(await page.evaluate(() => window.__stub.inits), 1, 'the loaded model is reused')

  // ---- A highlight is read by the model; another tab leaves it to this one --------------------
  await highlights.getByRole('switch').click()
  await closeSettings(page)
  // The top bar's button appears once the model is downloaded, and shows it is on.
  assert.equal(await topBar.getAttribute('aria-pressed'), 'true')
  const other = await context.newPage()
  other.setDefaultTimeout(8000)
  await other.addInitScript(fakeGpu)
  other.on('pageerror', (error) => errors.push(error.message))
  await other.goto(base)
  await other.waitForURL(`${base}/channel/general`)
  // The demo echo bot repeats each message, so a mention of your own nick comes back as a highlight.
  const bootstrap = await (await context.request.get(`${base}/api/v1/bootstrap`)).json()
  const nick = bootstrap.networks?.[0]?.nick
  assert.ok(nick, 'demo nick')
  // The other tab sends, and without the model lock it would claim the echo first and load its
  // own copy; the tab holding the model must read it instead.
  const otherComposer = other.getByRole('textbox', { name: 'Message #general', exact: true })
  delayEvents = 1500
  await otherComposer.fill(`@${nick} have a look at #dev!!!`)
  await otherComposer.press('Enter')
  await page.waitForFunction(() => window.__stub.requests.length === 3, null, { timeout: 8000 })
  delayEvents = 0
  requests = await page.evaluate(() => window.__stub.requests)
  assert.match(requests.at(-1).text, /^echo in general: .* have a look at dev!$/)
  await page.waitForTimeout(800)
  assert.equal((await page.evaluate(() => window.__stub.requests)).length, 3, 'read once')
  assert.equal(await other.evaluate(() => window.__workers.created), 0, 'one tab holds the model')

  // ---- A Test in the other tab loads a guest copy and frees it right after ----------------------
  const otherSettings = await openVoiceSettings(other)
  const otherTest = otherSettings.locator('[data-setting="local-test"]')
  await otherTest.getByRole('button', { name: 'Test', exact: true }).click()
  await otherTest.getByText('Played a sample.').waitFor()
  await other.waitForFunction(() => window.__workers.terminated === 2, null, { timeout: 3000 })
  assert.deepEqual(await other.evaluate(() => window.__workers), { created: 2, terminated: 2 })
  assert.deepEqual(await page.evaluate(() => window.__workers), { created: 2, terminated: 0 })
  await closeSettings(other)
  await page.bringToFront()
  const composer = page.getByRole('textbox', { name: 'Message #general', exact: true })

  // ---- Switching read-aloud off frees the model ------------------------------------------------
  await topBar.click()
  assert.deepEqual(await page.evaluate(() => window.__workers), { created: 2, terminated: 2 })

  // ---- No crossover: a model that cannot start reads nothing and calls no service ------------
  const preferences = await (await context.request.get(`${base}/api/app/preferences`)).json()
  const saved = await context.request.put(`${base}/api/app/preferences`, {
    headers: { origin: base },
    data: {
      preferences: {
        ...(preferences.preferences ?? preferences),
        speech: { baseUrl: 'https://speech.invalid/v1', model: 'tts-1', voice: 'alloy' },
      },
    },
  })
  assert.equal(saved.status(), 200)
  stubMode = 'fail'
  await other.close()
  await page.reload()
  await page.waitForURL(`${base}/channel/general`)
  await topBar.click()
  await composer.fill(`@${nick} are you there`)
  await composer.press('Enter')
  await page
    .getByRole('status')
    .filter({ hasText: /The voice model could not start on this browser: stub adapter lost/ })
    .waitFor()
  assert.deepEqual(speechRequests, [])

  // ---- Never downloads without a click ---------------------------------------------------------
  stubMode = 'ok'
  settings = await openVoiceSettings(page)
  await settings
    .locator('[data-setting="local-model"]')
    .getByRole('button', { name: 'Remove', exact: true })
    .click()
  await page
    .getByRole('dialog', { name: 'Remove the voice model?' })
    .getByRole('button', { name: 'Remove', exact: true })
    .click()
  await settings
    .locator('[data-setting="local-model"]')
    .getByText('Not downloaded. 921 MB from huggingface.co.')
    .waitFor()
  assert.deepEqual(
    await page.evaluate(async () =>
      (await caches.keys()).filter((name) => name.startsWith('zonos3-')),
    ),
    [],
  )
  await closeSettings(page)
  await page.evaluate(() => localStorage.setItem('ircord.device.readAloud', 'true'))
  await page.reload()
  await page.waitForURL(`${base}/channel/general`)
  await composer.fill(`@${nick} one more`)
  await composer.press('Enter')
  await page
    .getByRole('status')
    .filter({ hasText: 'Download the voice model in Settings → Read aloud first.' })
    .waitFor()
  assert.equal(await page.evaluate(() => window.__workers.created), 0)
  assert.deepEqual(speechRequests, [])

  // The offline search worker's WebAssembly policy errors are separate (see verify-workspace-pages).
  assert.deepEqual(
    errors.filter((error) => !error.includes('WebAssembly')),
    [],
  )
  console.log(
    'Voice model switch, downsides info, download, voices, Test, highlights, tab lock, unload, no fallback and no automatic download pass.',
  )
} catch (error) {
  // Server diagnostics exclude configuration and request bodies.
  if (serverLog) console.error(serverLog)
  throw error
} finally {
  if (created) await context?.request.delete(`${base}/api/v1/auth/account`).catch(() => {})
  await browser?.close()
  frontend.kill('SIGTERM')
}
