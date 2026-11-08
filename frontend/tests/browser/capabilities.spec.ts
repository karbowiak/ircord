import { test, expect } from '@playwright/test'
import { signIn } from './helpers'

test('message actions refresh on live feature changes, network switches and reconnects', async ({
  page,
}) => {
  let limited = false
  await page.routeWebSocket('**/api/v1/events*', (socket) => {
    const server = socket.connectToServer()
    server.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      if (event.type === 'network.updated' && event.data.id === 'libera' && limited) {
        event.data.features.replies = false
        event.data.features.reactions = false
        event.data.features.typing = false
        event.data.features.redact = false
        delete event.data.features.multiline
      }
      socket.send(JSON.stringify(event))
    })
  })
  await signIn(page)
  const latest = page
    .locator('.message-row')
    .filter({ hasText: 'is now very much planning a road trip' })
  await latest.hover()
  await expect(latest.getByRole('button', { name: 'Reply to message' })).toBeEnabled()
  await expect(latest.getByRole('button', { name: 'Add reaction', exact: true })).toBeEnabled()
  await latest.click({ button: 'right' })
  await expect(page.getByRole('menuitem', { name: 'Reply', exact: true })).toBeVisible()

  // A real sequenced network.updated frame closes the stale menu and refreshes
  // controls in place; no reload or conversation switch is required.
  limited = true
  const state = await (await page.request.get('/api/v1/bootstrap')).json()
  const network = state.networks.find((n: { id: string }) => n.id === 'libera')
  const config = {
    name: network.name,
    host: network.host,
    port: network.port,
    tls: network.tls,
    nick: network.nick,
    auth: { method: 'none' },
    autoJoin: network.autoJoin,
    perform: [],
  }
  expect((await page.request.patch('/api/v1/networks/libera', { data: config })).ok()).toBe(true)
  await expect(page.getByRole('menu')).toHaveCount(0)
  await latest.hover()
  await expect(latest.getByRole('button', { name: 'Reply to message' })).toHaveCount(0)
  await expect(latest.getByRole('button', { name: 'Add reaction', exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'Attach or add' }).click()
  await expect(page.getByRole('menuitem', { name: /Upload a file/ })).not.toHaveAttribute(
    'aria-disabled',
    'true',
  )
  await page.keyboard.press('Escape')

  const input = page.getByRole('combobox', { name: 'Message #general' })
  const draft = 'keep this draft\non an unsupported network'
  await input.fill(draft)
  await input.press('Enter')
  await expect(input).toHaveValue(draft)
  await expect(page.getByText(/This network does not support multiline messages/)).toBeVisible()
  await expect(page.locator('.is-pending')).toHaveCount(0)

  // Another network keeps its own features.
  await page.getByRole('button', { name: 'Ergo', exact: true }).click()
  await page.getByRole('button', { name: 'Add a network', exact: true }).focus() // leave composer focus
  const ergoInput = page.getByRole('combobox')
  await expect(ergoInput).toBeEnabled()
  await page.getByRole('button', { name: 'Libera Chat', exact: true }).click()
  await expect(input).toHaveValue(draft)

  await input.fill('')
  limited = false
  await page.request.patch('/api/v1/networks/libera', { data: { status: 'disconnected' } })
  await expect(input).toBeDisabled()
  await page.request.patch('/api/v1/networks/libera', { data: { status: 'connected' } })
  await expect(input).toBeEnabled()
  await latest.hover()
  await expect(latest.getByRole('button', { name: 'Reply to message' })).toBeEnabled()
  await page.screenshot({ path: 'test-results/capabilities-desktop.png' })
})

test('unsupported actions retain history and work on small screens', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.route('**/api/v1/bootstrap', async (route) => {
    const response = await route.fetch()
    const body = await response.json()
    for (const n of body.networks) {
      n.features.replies = false
      n.features.reactions = false
    }
    await route.fulfill({ response, json: body })
  })
  await signIn(page)
  const row = page
    .locator('.message-row')
    .filter({ hasText: 'is now very much planning a road trip' })
  await row.click({ button: 'right' })
  await expect(page.getByRole('menuitem', { name: 'Reply', exact: true })).toHaveCount(0)
  await expect(page.getByRole('menuitem', { name: /Add reaction/ })).toHaveCount(0)
  await expect(page.locator('.reactions .reaction')).not.toHaveCount(0)
  await expect(page.locator('.reactions button')).toHaveCount(0)
  await expect(page.getByRole('menuitem', { name: 'Copy text', exact: true })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
  await page.screenshot({ path: 'test-results/capabilities-mobile.png' })
})
