import { expect, type Page } from '@playwright/test'

/** Creates a fresh key account (each test gets its own) and opens the workspace. */
export async function signIn(page: Page) {
  await page.goto('/')
  await page.getByRole('button', { name: 'Create an account' }).click()
  await page.getByLabel('I’ve saved my key somewhere safe').check()
  await page.getByRole('button', { name: 'Continue to IRCord' }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
}
