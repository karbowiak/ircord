import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { signIn } from './helpers'

const openDetails = async (page: Page) => {
  await page.getByRole('button', { name: 'Server', exact: true }).click()
  await page.getByRole('button', { name: 'Server capabilities and connection details' }).click()
  return page.getByRole('region', { name: 'Server connection' })
}

test('the Server panel explains negotiated and announced capabilities, limits and TLS', async ({
  page,
}) => {
  await signIn(page)
  const panel = await openDetails(page)
  expect(
    (await panel.getByRole('searchbox', { name: 'Filter capabilities' }).boundingBox())!.width,
  ).toBeGreaterThan(200)
  const sasl = panel.locator('li').filter({ has: page.getByText('sasl', { exact: true }) })
  await expect(sasl).toContainText('Announced')
  await expect(sasl).toContainText('PLAIN,EXTERNAL')
  const multiline = panel
    .locator('li')
    .filter({ has: page.getByText('draft/multiline', { exact: true }) })
  await expect(multiline).toContainText('Active')
  await expect(multiline).toContainText('max-bytes=4096,max-lines=100')
  await panel.getByLabel('Capability status').selectOption('offered')
  await expect(multiline).toHaveCount(0)
  await expect(sasl).toBeVisible()
  await panel.getByLabel('Capability status').selectOption('all')
  await panel.getByRole('searchbox', { name: 'Filter capabilities' }).fill('max-bytes')
  await expect(panel.locator('li')).toHaveCount(1)
  await panel.getByRole('searchbox', { name: 'Filter capabilities' }).fill('no-such-extension')
  await expect(panel).toContainText('No capabilities match your search.')
  await panel.getByRole('button', { name: 'ISUPPORT', exact: true }).click()
  await panel.getByRole('searchbox', { name: 'Filter ISUPPORT' }).fill('PREFIX')
  await expect(panel.locator('dd')).toHaveText('(qaohv)~&@%+')
  await panel.getByRole('button', { name: 'Connection', exact: true }).click()
  await expect(panel).toContainText('irc.libera.chat:6697')
  await expect(panel).toContainText('TLS 1.3 · Certificate verified')
  await expect(page.getByRole('log', { name: /server messages/ })).toBeVisible()
  expect(
    (
      await new AxeBuilder({ page })
        .include('.server-connection')
        .withTags(['wcag2a', 'wcag2aa'])
        .analyze()
    ).violations.map((v) => v.id),
  ).toEqual([])
  await panel.getByRole('button', { name: 'Capabilities', exact: true }).click()
  await page.screenshot({ path: 'test-results/server-capabilities-desktop.png' })
  await panel
    .getByRole('button', { name: 'Server capabilities and connection details' })
    .press('Enter')
  await expect(panel.getByRole('navigation', { name: 'Connection information' })).toHaveCount(0)
})

test('connection details follow live frames and network switches without polling', async ({
  page,
}) => {
  let count = 0
  page.on('request', (r) => {
    if (r.url().endsWith('/networks/libera/connection')) count++
  })
  await page.routeWebSocket('**/api/v1/events*', (socket) => {
    const server = socket.connectToServer()
    server.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      if (event.type === 'connection.updated' && event.data.networkId === 'libera') {
        event.data.offeredCapabilities['draft/new-extension'] = 'limit=12'
        event.data.negotiatedCapabilities.push('draft/new-extension')
      }
      socket.send(JSON.stringify(event))
    })
  })
  await signIn(page)
  let panel = await openDetails(page)
  await expect(panel.locator('li')).not.toHaveCount(0)
  expect(count).toBe(1)
  await page.request.patch('/api/v1/networks/libera', { data: { status: 'disconnected' } })
  await expect(panel).toContainText('Capabilities become active when this network connects.')
  await expect(panel).toContainText('Details from the last connection.')
  await page.request.patch('/api/v1/networks/libera', { data: { status: 'connected' } })
  await expect(panel.locator('.server-capability-strip')).toContainText('draft/new-extension')
  expect(count).toBe(1)
  await page.getByRole('button', { name: 'OFTC', exact: true }).click()
  panel = await openDetails(page)
  await expect(panel).toContainText('irc.oftc.net')
  await expect(panel).not.toContainText('draft/new-extension')
  await expect(panel.locator('.server-capability-strip')).not.toContainText('draft/multiline')
})

test('slow and failed handshakes keep the log usable and can be retried', async ({ page }) => {
  let fail = true
  await page.route('**/api/v1/networks/libera/connection', async (route) => {
    if (fail) return route.fulfill({ status: 503, json: { statusMessage: 'Unavailable' } })
    await route.continue()
  })
  await signIn(page)
  const panel = await openDetails(page)
  await expect(panel).toContainText('Couldn’t load the connection details.')
  await expect(page.getByRole('log', { name: /server messages/ })).toBeVisible()
  fail = false
  await panel.getByRole('button', { name: 'Try again' }).click()
  await expect(panel).toContainText('PLAIN,EXTERNAL')
})

test('a late HTTP snapshot cannot replace a newer private connection event', async ({ page }) => {
  let release!: () => void
  let received!: () => void
  const waiting = new Promise<void>((resolve) => {
    received = resolve
  })
  const hold = new Promise<void>((resolve) => {
    release = resolve
  })
  await page.route('**/api/v1/networks/libera/connection', async (route) => {
    const response = await route.fetch()
    const stale = await response.json()
    stale.serverName = 'old-snapshot.example'
    received()
    await hold
    await route.fulfill({ response, json: stale }).catch(() => {})
  })
  await signIn(page)
  const opening = openDetails(page)
  await waiting
  await expect(page.getByText('Loading the server handshake…')).toBeVisible()
  await page.request.patch('/api/v1/networks/libera', { data: { status: 'disconnected' } })
  const panel = await opening
  await expect(panel).toContainText('Details from the last connection.')
  release()
  await expect(panel).not.toContainText('old-snapshot.example')
  await expect(panel).toContainText('irc.libera.chat')
})

test('capability details fit a phone and keep the server log visible', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await signIn(page)
  await page.getByRole('button', { name: 'Open conversations' }).click()
  const panel = await openDetails(page)
  await expect(panel.getByRole('searchbox', { name: 'Filter capabilities' })).toBeVisible()
  await expect(panel.locator('.connection-state-label')).toHaveText('Connected')
  await expect(panel.locator('.connection-state-label')).toBeVisible()
  await expect(
    panel.getByRole('button', { name: 'Server capabilities and connection details' }),
  ).toHaveAccessibleName('Server capabilities and connection details (Connected)')
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
  expect((await page.locator('.server-log').boundingBox())!.height).toBeGreaterThan(120)
  expect(
    (await page.locator('.server-line .server-text').last().boundingBox())!.width,
  ).toBeGreaterThan(200)
  await page.screenshot({ path: 'test-results/server-capabilities-mobile.png' })
  for (const [height, minimumLog, name] of [
    [667, 120, 'short'],
    [440, 60, 'keyboard'],
  ] as const) {
    await page.setViewportSize({ width: 390, height })
    await expect(panel.locator('.connection-state-label')).toBeVisible()
    const log = (await page.locator('.server-log').boundingBox())!
    const workspace = (await page.locator('.server-window').boundingBox())!
    const summary = (await panel.boundingBox())!
    expect(log.height).toBeGreaterThan(minimumLog)
    expect(summary.height).toBeLessThanOrEqual(workspace.height * 0.6 + 1)
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
    await page.screenshot({ path: `test-results/server-capabilities-mobile-${name}.png` })
  }
  await page.request.patch('/api/v1/networks/libera', { data: { status: 'disconnected' } })
  await expect(panel.locator('.connection-state-label')).toHaveText('Disconnected')
  await expect(
    panel.getByRole('button', { name: 'Server capabilities and connection details' }),
  ).toHaveAccessibleName('Server capabilities and connection details (Disconnected)')
})
