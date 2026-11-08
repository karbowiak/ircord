import { test, expect } from '@playwright/test'
import { signIn } from './helpers'

test('/ignore hides someone, lists them, and /unignore with Tab brings them back', async ({
  page,
}) => {
  await signIn(page)
  const input = page.getByRole('combobox', { name: 'Message #general' })
  const coffee = page.locator('.message-row').filter({ hasText: 'Coffee is on' })
  const notice = (text: string | RegExp) => page.getByText(text).last()
  await expect(coffee).toHaveCount(1)

  // A trailing space keeps the command menu closed, so Enter submits.
  await input.fill('/ignore Mika ')
  await input.press('Enter')
  await expect(notice(/Ignoring Mika\. Their messages are hidden/)).toBeVisible()
  await expect(coffee).toHaveCount(0)

  await input.fill('/ignore ')
  await input.press('Enter')
  await expect(notice('Ignoring 1 person: Mika')).toBeVisible()

  await input.fill('/unignore Mi')
  await input.press('Tab')
  await expect(input).toHaveValue('/unignore Mika ')
  await input.press('Enter')
  await expect(notice('No longer ignoring Mika.')).toBeVisible()
  await expect(coffee).toHaveCount(1)
})
