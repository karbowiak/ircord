import { expect, type Locator, type Page } from '@playwright/test'

let count = 0
/** A username nobody else has: on the demo network, and across test runs. */
export const uniqueUsername = () =>
  `t${Date.now().toString(36)}${(count++).toString(36)}${Math.floor(Math.random() * 1296).toString(36)}`
export const testPassword = 'correct-horse-battery'

/** Signs up a fresh account (each test gets its own) and opens the workspace. */
export async function signIn(page: Page, username = uniqueUsername()) {
  await page.goto('/')
  await page.getByRole('button', { name: 'Create an account' }).click()
  await page.getByLabel('Username', { exact: true }).fill(username)
  await expect(page.getByText(`${username} is available`)).toBeVisible()
  await page.getByLabel('Password', { exact: true }).fill(testPassword)
  await page.getByRole('button', { name: 'Create account', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  return username
}

/** Picks an option in an AppSelect (a combobox button with a listbox popup). */
export async function chooseOption(page: Page, combobox: Locator, option: string) {
  await combobox.click()
  await page.getByRole('listbox').getByRole('option', { name: option, exact: true }).click()
  await expect(page.getByRole('listbox')).toBeHidden()
}
