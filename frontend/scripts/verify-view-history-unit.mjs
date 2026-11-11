// Real browser History API + Vue Router, with no app account or backend.
import assert from 'node:assert/strict'
import { build } from 'esbuild'
import { chromium, webkit } from 'playwright'

const bundle = await build({
  stdin: {
    contents: `
      import { ref, watch, nextTick } from 'vue'
      import { createRouter, createWebHistory } from 'vue-router'
      import { createViewHistory, installViewHistoryEvents } from './app/utils/view-history.ts'
      installViewHistoryEvents()
      const router = createRouter({ history: createWebHistory(), routes: [
        {path: '/chat', component: {}}, {path: '/search', component: {}}
      ] })
      await router.push('/chat')
      const views = createViewHistory(router)
      const outer = ref(false), inner = ref(false), tab = ref('appearance')
      let blocked = false
      views.track('outer', outer)
      views.track('inner', inner)
      views.track('section', tab)
      for (const state of [outer, inner]) {
        let unregister
        watch(state, (value) => {
          unregister?.()
          unregister = value ? views.register(() => {
            if (blocked) return false
            state.value = false
            return true
          }, () => !blocked) : undefined
        }, {flush: 'sync'})
      }
      window.fixture = {
        open: () => { outer.value = true }, nested: () => { inner.value = true },
        tab: (value) => { tab.value = value }, close: () => views.closeTop(),
        block: (value) => { blocked = value }, push: () => router.push('/search'),
        rapid: () => { outer.value = false; setTimeout(() => { outer.value = true }, 0) },
        state: () => ({ outer: outer.value, inner: inner.value, tab: tab.value,
          count: views.count, position: history.state.position, length: history.length,
          route: router.currentRoute.value.fullPath, values: history.state.ircordViews?.values }),
        dispose: () => views.dispose()
      }
    `,
    resolveDir: process.cwd(),
    sourcefile: 'view-history-fixture.ts',
    loader: 'ts',
  },
  bundle: true,
  write: false,
  format: 'esm',
  target: 'esnext',
})
const browser = await (process.env.ENGINE === 'webkit' ? webkit : chromium).launch()
try {
  const page = await browser.newPage()
  const errors = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.route('http://history.test/**', (route) =>
    route.fulfill({
      contentType: 'text/html',
      body: `<script type="module">${bundle.outputFiles[0].text}</script>`,
    }),
  )
  await page.goto('http://history.test/chat')
  await page.waitForFunction(() => !!window.fixture)
  const state = () => page.evaluate(() => window.fixture.state())
  const call = async (method, argument) => {
    await page.evaluate(
      ([method, argument]) => window.fixture[method](argument),
      [method, argument],
    )
    await page.waitForTimeout(30)
  }
  const move = async (method) => {
    await page.evaluate((method) => history[method](), method)
    await page.waitForTimeout(60)
  }
  const initial = await state()
  await call('open')
  assert.equal((await state()).position, initial.position + 1)
  await call('tab', 'profile')
  await call('tab', 'notifications')
  await move('back')
  assert.equal((await state()).tab, 'profile')
  await move('forward')
  assert.equal((await state()).tab, 'notifications')
  await call('nested')
  await move('back')
  assert.equal((await state()).inner, false)
  assert.equal((await state()).outer, true)
  await move('forward')
  assert.equal((await state()).inner, true)
  await call('block', true)
  const guarded = await state()
  await move('back')
  assert.deepEqual(await state(), guarded, 'A refused close restores the entry')
  await call('close')
  assert.deepEqual(await state(), guarded, 'An explicit close respects the same guard')
  await call('push')
  assert.deepEqual(await state(), guarded, 'Route navigation respects the close guard')
  await call('block', false)
  await call('close')
  assert.equal((await state()).inner, false)
  await call('close')
  assert.equal((await state()).position, initial.position, 'Explicit close consumes all sections')
  const length = (await state()).length
  for (let count = 0; count < 12; count++) {
    await call('open')
    await call('close')
    assert.equal((await state()).position, initial.position)
  }
  assert((await state()).length <= length, 'History does not grow across repeated open/close')
  await call('open')
  await call('rapid')
  assert.equal((await state()).outer, true)
  assert.equal((await state()).position, initial.position + 1, 'Rapid reopening has its own entry')
  await move('back')
  assert.equal((await state()).outer, false)
  assert.equal((await state()).position, initial.position)
  await call('open')
  await call('tab', 'profile')
  await call('push')
  assert.equal((await state()).route, '/search')
  await move('back')
  assert.equal((await state()).route, '/chat')
  await move('forward')
  assert.equal((await state()).route, '/search')
  assert.deepEqual(errors, [])
  await call('dispose')
  console.log(
    'Shared view history: nested views, tabs, guards, repeated closure, routes and Forward pass.',
  )
} finally {
  await browser.close()
}
