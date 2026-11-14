// Disposable demo: delay an already committed response behind a newer live event.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { createServer } from 'node:net'
import { spawn } from 'node:child_process'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const frontendDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const probe = createServer()
await new Promise((resolve) => probe.listen(0, '127.0.0.1', resolve))
const port = probe.address().port
await new Promise((resolve) => probe.close(resolve))
const base = `http://127.0.0.1:${port}`
const frontend = spawn(
  process.execPath,
  [
    '-e',
    `
  import { runCommand } from '@nuxt/cli'
  await runCommand('dev', ['--host', '127.0.0.1', '--port', '${port}', '--strictPort', '--no-fork', '--no-tui', '--no-takeover'], {
    overrides: { buildDir: '.data/nuxt-profile-completion', typesDir: '.data/nuxt-profile-completion', nitro: { devStorage: { data: { driver: 'memory' } } } },
  })
`,
  ],
  {
    cwd: frontendDir,
    env: { ...process.env, NUXT_DEMO: 'true', NUXT_BACKEND_URL: 'http://127.0.0.1:1' },
    stdio: ['ignore', 'pipe', 'pipe'],
  },
)
let serverLog = ''
for (const stream of [frontend.stdout, frontend.stderr])
  stream.on('data', (chunk) => (serverLog = (serverLog + chunk).slice(-4000)))
let browser
let context
const barriers = new Set()
const deferred = () => {
  let resolve
  const promise = new Promise((done) => {
    resolve = done
  })
  return { promise, resolve }
}
const bounded = (promise) => {
  let timer
  return Promise.race([
    promise,
    new Promise((_, reject) => {
      timer = setTimeout(
        () => reject(new Error('Profile request did not reach the response barrier')),
        7000,
      )
    }),
  ]).finally(() => clearTimeout(timer))
}
try {
  for (let attempt = 0; ; attempt++) {
    try {
      if ((await fetch(base)).ok) break
    } catch {
      /* Still starting. */
    }
    assert(attempt < 180 && frontend.exitCode === null, `Demo did not start: ${serverLog}`)
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  browser = await chromium.launch({ headless: true })
  context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `profile${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  const bootstrap = await (await context.request.get(`${base}/api/v1/bootstrap`)).json()
  assert.equal(bootstrap.demo, true)
  const networkID = bootstrap.networks[0].id
  const profilePath = `/api/v1/networks/${encodeURIComponent(networkID)}/profile`
  const page = await context.newPage()
  page.setDefaultTimeout(7000)
  const errors = []
  page.on('pageerror', (error) => errors.push(error.message))
  const requests = []
  let gate
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (
      route.request().method() !== 'PATCH' ||
      (path !== profilePath && path !== '/api/v1/profile')
    )
      return route.continue()
    requests.push({ path, body: route.request().postDataJSON() })
    const response = await route.fetch()
    const held = gate
    if (held) {
      gate = undefined
      held.committed.resolve()
      await held.release.promise
    }
    await route.fulfill({ response })
  })
  const holdNext = () => {
    gate = { committed: deferred(), release: deferred() }
    barriers.add(gate)
    return gate
  }
  const storeProfile = (id) => {
    const chat = document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
    return id ? chat.data.networks.find((network) => network.id === id)?.profile : chat.data.user
  }
  await page.goto(base)
  await page.waitForFunction(
    () =>
      document.querySelector('#__nuxt')?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')
        ?.status === 'connected',
  )
  await page.getByRole('button', { name: 'User settings' }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings.getByRole('button', { name: 'Profile', exact: true }).click()
  const name = page.locator('#profile-name')
  const bio = page.locator('#profile-bio')
  const save = page.locator('.profile-form').getByRole('button', { name: 'Save changes' })
  await name.waitFor()

  // A has committed, but its response is held; B arrives over the live socket.
  await bio.fill('Committed A')
  const first = holdNext()
  await save.click()
  await bounded(first.committed.promise)
  assert.deepEqual(requests.at(-1).body, { bio: 'Committed A' })
  const newer = await context.request.patch(base + profilePath, {
    data: { displayName: 'Newer B' },
  })
  assert.equal(newer.status(), 200)
  await page.waitForFunction(
    ([id, expected]) => {
      const chat = document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      return (
        chat.data.networks.find((network) => network.id === id)?.profile?.displayName === expected
      )
    },
    [networkID, 'Newer B'],
  )
  await name.waitFor()
  assert.equal(await name.inputValue(), 'Newer B')
  await bio.fill('Draft during save')
  first.release.resolve()
  await page.waitForFunction(() =>
    [...document.querySelectorAll('.profile-form button')].some(
      (button) => button.textContent?.trim() === 'Save changes' && !button.disabled,
    ),
  )
  assert.equal((await page.evaluate(storeProfile, networkID)).displayName, 'Newer B')
  assert.equal(await name.inputValue(), 'Newer B')
  assert.equal(await bio.inputValue(), 'Draft during save')
  await bio.fill('Committed A')
  assert.equal(await save.isDisabled(), true, 'Untouched B must not become a dirty field')
  await bio.fill('Later unrelated edit')
  await save.click()
  await page.waitForFunction(() =>
    [...document.querySelectorAll('.profile-form button')].some(
      (button) => button.textContent?.trim() === 'Save changes' && button.disabled,
    ),
  )
  assert.deepEqual(requests.at(-1).body, { bio: 'Later unrelated edit' })
  const finalNetwork = await (await context.request.get(base + profilePath)).json()
  assert.equal(finalNetwork.displayName, 'Newer B')
  assert.equal(finalNetwork.bio, 'Later unrelated edit')

  // B restores the original value after A committed. Its event must still win.
  await bio.fill('Same-field A')
  const same = holdNext()
  await save.click()
  await bounded(same.committed.promise)
  assert.equal(
    (
      await context.request.patch(base + profilePath, { data: { bio: 'Later unrelated edit' } })
    ).status(),
    200,
  )
  await page.waitForFunction(
    ([id, expected]) => {
      const chat = document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      return chat.data.networks.find((network) => network.id === id)?.profile?.bio === expected
    },
    [networkID, 'Later unrelated edit'],
  )
  same.release.resolve()
  await page.waitForFunction(
    () => !document.querySelector('.profile-form')?.textContent?.includes('Saving'),
  )
  assert.equal((await page.evaluate(storeProfile, networkID)).bio, 'Later unrelated edit')
  assert.equal(await bio.inputValue(), 'Later unrelated edit')
  assert.equal(await save.isDisabled(), true)

  // The legacy account profile mutation uses the same completion guard.
  const account = holdNext()
  await page.evaluate(() => {
    const chat = document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
    window.__profileCompletion = chat.profile.mutateAsync({ bio: 'Account A' })
  })
  await bounded(account.committed.promise)
  assert.equal(
    (
      await context.request.patch(`${base}/api/v1/profile`, { data: { displayName: 'Account B' } })
    ).status(),
    200,
  )
  await page.waitForFunction(
    () =>
      document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$pinia._s.get('chat')
        .data.user.displayName === 'Account B',
  )
  account.release.resolve()
  const accountResult = await page.evaluate(() => window.__profileCompletion)
  assert.equal(accountResult.displayName, 'Account B')
  assert.equal((await page.evaluate(storeProfile)).displayName, 'Account B')

  // A client without delivered profile events still applies successful HTTP responses.
  const quiet = await browser.newContext({ serviceWorkers: 'block' })
  try {
    const signupQuiet = await quiet.request.post(`${base}/api/v1/auth/signup`, {
      data: { username: `quiet${Date.now().toString(36)}`, password: randomUUID() },
    })
    assert.equal(signupQuiet.status(), 201)
    const quietBootstrap = await (await quiet.request.get(`${base}/api/v1/bootstrap`)).json()
    const quietNetwork = quietBootstrap.networks[0].id
    await quiet.routeWebSocket('**/api/v1/events**', (socket) => {
      socket.send(
        JSON.stringify({ v: 1, type: 'session.ready', cursor: quietBootstrap.cursor, sync: false }),
      )
    })
    const quietPage = await quiet.newPage()
    await quietPage.goto(base)
    await quietPage.waitForFunction(
      () =>
        document
          .querySelector('#__nuxt')
          ?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')?.status === 'connected',
    )
    const result = await quietPage.evaluate(async (id) => {
      const chat = document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      const network = await chat.networkProfile.mutateAsync({ id, input: { bio: 'HTTP network' } })
      const user = await chat.profile.mutateAsync({ bio: 'HTTP account' })
      return {
        network: network.bio,
        storedNetwork: chat.data.networks.find((item) => item.id === id)?.profile?.bio,
        user: user.bio,
        storedUser: chat.data.user.bio,
      }
    }, quietNetwork)
    assert.deepEqual(result, {
      network: 'HTTP network',
      storedNetwork: 'HTTP network',
      user: 'HTTP account',
      storedUser: 'HTTP account',
    })
  } finally {
    await quiet.close()
  }
  assert.deepEqual(errors, [])
  console.log('Profile completion race checks passed')
} finally {
  for (const barrier of barriers) barrier.release.resolve()
  await context?.close()
  await browser?.close()
  if (frontend.exitCode === null && frontend.signalCode === null) {
    const exited = new Promise((resolve) => frontend.once('exit', resolve))
    frontend.kill('SIGTERM')
    await exited
  }
}
